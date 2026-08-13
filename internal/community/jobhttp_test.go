package community

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

func jobAgent(t *testing.T, executor JobExecutor) *Agent {
	t.Helper()
	agent := &Agent{
		cfg:          Config{Definition: Definition{MaxContextMessages: 12}},
		systemPrompt: "policy",
		telemetry:    telemetryOrNoop(nil),
		slots:        make(chan struct{}, 1),
	}
	agent.ensureRuntimeDefaults()
	agent.jobs = &JobRunner{
		Store:     NewMemoryJobStore(nil),
		Executors: map[string]JobExecutor{"echo": executor},
		Timeout:   5 * time.Second,
	}
	if err := agent.jobs.Start(context.Background()); err != nil {
		t.Fatalf("Start: %v", err)
	}
	t.Cleanup(agent.jobs.Stop)
	return agent
}

func postJob(t *testing.T, agent *Agent, body string) (*httptest.ResponseRecorder, jobView) {
	t.Helper()
	request := httptest.NewRequest(http.MethodPost, "/v1/jobs", strings.NewReader(body))
	request.Header.Set("Content-Type", "application/json")
	request.Header.Set("X-Sirens-Caller", "tester")
	recorder := httptest.NewRecorder()
	agent.HTTPHandler().ServeHTTP(recorder, request)
	var view jobView
	if recorder.Code == http.StatusAccepted {
		if err := json.Unmarshal(recorder.Body.Bytes(), &view); err != nil {
			t.Fatalf("decode: %v, body = %s", err, recorder.Body.String())
		}
	}
	return recorder, view
}

// Submit returns an id immediately rather than a completed answer.
func TestHTTPSubmitReturnsAnIDWithoutHoldingTheRequest(t *testing.T) {
	t.Parallel()
	blocker := newBlockingExecutor()
	agent := jobAgent(t, blocker)
	t.Cleanup(func() { close(blocker.release) })

	recorder, view := postJob(t, agent, `{"kind":"echo","request_id":"first"}`)
	if recorder.Code != http.StatusAccepted {
		t.Fatalf("status = %d, body = %s", recorder.Code, recorder.Body.String())
	}
	if view.ID == "" || view.State != string(JobQueued) {
		t.Fatalf("view = %#v", view)
	}
}

func TestHTTPPollReportsStateAndCancelStops(t *testing.T) {
	t.Parallel()
	blocker := newBlockingExecutor()
	agent := jobAgent(t, blocker)

	_, submitted := postJob(t, agent, `{"kind":"echo","request_id":"pollable"}`)
	select {
	case <-blocker.started:
	case <-time.After(5 * time.Second):
		t.Fatal("the job never started")
	}

	poll := func() jobView {
		t.Helper()
		request := httptest.NewRequest(http.MethodGet, jobsPath+submitted.ID, nil)
		request.Header.Set("X-Sirens-Caller", "tester")
		recorder := httptest.NewRecorder()
		agent.HTTPHandler().ServeHTTP(recorder, request)
		if recorder.Code != http.StatusOK {
			t.Fatalf("poll status = %d, body = %s", recorder.Code, recorder.Body.String())
		}
		var view jobView
		if err := json.Unmarshal(recorder.Body.Bytes(), &view); err != nil {
			t.Fatalf("decode poll: %v", err)
		}
		return view
	}
	if state := poll().State; state != string(JobRunning) && state != string(JobQueued) {
		t.Errorf("polled state = %s", state)
	}

	cancelRequest := httptest.NewRequest(http.MethodPost, jobsPath+submitted.ID+"/cancel", nil)
	cancelRequest.Header.Set("X-Sirens-Caller", "tester")
	cancelRecorder := httptest.NewRecorder()
	agent.HTTPHandler().ServeHTTP(cancelRecorder, cancelRequest)
	if cancelRecorder.Code != http.StatusOK {
		t.Fatalf("cancel status = %d, body = %s", cancelRecorder.Code, cancelRecorder.Body.String())
	}

	deadline := time.Now().Add(5 * time.Second)
	for time.Now().Before(deadline) {
		if poll().State == string(JobCancelled) {
			close(blocker.release)
			return
		}
		time.Sleep(10 * time.Millisecond)
	}
	close(blocker.release)
	t.Fatalf("job never reached cancelled, last state %s", poll().State)
}

// Another principal's job is indistinguishable from one that does not exist,
// so an id cannot be probed for.
func TestHTTPHidesAnotherPrincipalsJob(t *testing.T) {
	t.Parallel()
	agent := jobAgent(t, EchoJobExecutor{})
	_, submitted := postJob(t, agent, `{"kind":"echo","request_id":"private"}`)

	request := httptest.NewRequest(http.MethodGet, jobsPath+submitted.ID, nil)
	request.Header.Set("X-Sirens-Caller", "someone-else")
	recorder := httptest.NewRecorder()
	agent.HTTPHandler().ServeHTTP(recorder, request)
	if recorder.Code != http.StatusNotFound {
		t.Errorf("status for another principal = %d, want 404", recorder.Code)
	}
	if strings.Contains(recorder.Body.String(), "another") {
		t.Errorf("body distinguishes ownership: %s", recorder.Body.String())
	}
}

func TestHTTPRejectsAnUndeclaredKind(t *testing.T) {
	t.Parallel()
	agent := jobAgent(t, EchoJobExecutor{})
	recorder, _ := postJob(t, agent, `{"kind":"deploy-everything"}`)
	if recorder.Code != http.StatusBadRequest {
		t.Errorf("status for an unknown kind = %d", recorder.Code)
	}
}

// Job submission shares the body cap with the turn endpoint and had the same
// collapse of every decode error into one message. See sirens-echo#351.
func TestHTTPSubmitSeparatesAnOversizeBodyFromABrokenOne(t *testing.T) {
	t.Parallel()
	agent := jobAgent(t, EchoJobExecutor{})

	oversized, _ := postJob(t, agent, `{"kind":"echo","idempotency_key":"`+
		strings.Repeat("k", maxHTTPBody+1)+`"}`)
	if oversized.Code != http.StatusBadRequest {
		t.Fatalf("oversize status = %d, body = %s", oversized.Code, oversized.Body.String())
	}
	if got := oversized.Body.String(); !strings.Contains(got, oversizeBodyMessage) {
		t.Errorf("oversize message = %q, want it to name the byte limit", got)
	}

	malformed, _ := postJob(t, agent, `{"kind":`)
	if malformed.Code != http.StatusBadRequest {
		t.Fatalf("malformed status = %d, body = %s", malformed.Code, malformed.Body.String())
	}
	if got := malformed.Body.String(); !strings.Contains(got, "must be a JSON object") {
		t.Errorf("malformed message = %q, want it to say malformed", got)
	}
}

// Two identical submissions are one job, which is the HTTP half of dedup.
func TestHTTPSubmitIsIdempotentOnItsKey(t *testing.T) {
	t.Parallel()
	agent := jobAgent(t, EchoJobExecutor{})
	body := `{"kind":"echo","idempotency_key":"the-same-request"}`
	_, first := postJob(t, agent, body)
	_, second := postJob(t, agent, body)
	if first.ID != second.ID {
		t.Errorf("same key made two jobs: %s and %s", first.ID, second.ID)
	}
}

func TestJobTerminalNoticeUsesTheHarnessFormat(t *testing.T) {
	t.Parallel()
	for state, want := range map[JobState]string{
		JobSucceeded: "finished",
		JobCancelled: "cancelled",
		JobFailed:    "failed",
	} {
		notice := jobTerminalNotice(Job{ID: "job-0123456789", State: state})
		if !noticeShape.MatchString(notice) {
			t.Errorf("%s notice %q does not match the harness shape", state, notice)
		}
		if !strings.Contains(notice, want) || !strings.Contains(notice, "job-0123456789") {
			t.Errorf("%s notice = %q", state, notice)
		}
	}
}

// Every job refusal used to be a bare http.Error, so sirens_echo.failures
// omitted this surface entirely. See sirens-echo#383.
func TestEveryJobRefusalCarriesItsOwnExceptionCode(t *testing.T) {
	t.Parallel()
	seen := map[string]exceptionSpec{}
	for _, code := range []exceptionCode{
		exceptionJobRequestInvalid,
		exceptionJobBodyTooLarge,
		exceptionJobRejected,
		exceptionJobNotFound,
		exceptionJobQueueFull,
	} {
		spec := exceptionFor(code)
		if spec.stage != "jobs" {
			t.Errorf("%s stage = %q, want jobs", spec.typeName, spec.stage)
		}
		if spec.fault == "" {
			t.Errorf("%s declares no fault, so it reads as the service's by default",
				spec.typeName)
		}
		if previous, exists := seen[spec.outcome]; exists {
			t.Errorf("%s shares outcome %q with %s, so the two cannot be separated",
				spec.typeName, spec.outcome, previous.typeName)
		}
		seen[spec.outcome] = spec
	}
	// The split issue 159 needs: a full queue is ours, the rest are the caller's.
	if got := exceptionFor(exceptionJobQueueFull).fault; got != faultService {
		t.Errorf("queue_full fault = %q, want %q", got, faultService)
	}
	for _, code := range []exceptionCode{
		exceptionJobRequestInvalid,
		exceptionJobBodyTooLarge,
		exceptionJobRejected,
		exceptionJobNotFound,
	} {
		if got := exceptionFor(code).fault; got != faultCaller {
			t.Errorf("%s fault = %q, want %q", exceptionFor(code).typeName, got, faultCaller)
		}
	}
}
