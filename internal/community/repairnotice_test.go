package community

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
	"time"
)

// A turn whose model calls all returned 200 must never report an outage. That
// is acceptance criterion 1 of sirens-echo#651.

// The reply from that trace, verbatim. It is correct, and the neutral rule
// refuses it for first-person voice, which is the argument the issue is about.
const firstPersonAnswer = "No, I do not have access to an AOS general knowledge " +
	"base. My available tools are limited to Eco server data, Forgejo issue " +
	"tracking, Steam store data, and scratchpad file operations."

// refusingServer answers every call with 200 and the same refusable reply, so
// the repair path exhausts without the backend ever failing.
func refusingServer(t *testing.T, calls *atomic.Int32) *httptest.Server {
	t.Helper()
	return httptest.NewServer(http.HandlerFunc(func(
		writer http.ResponseWriter,
		request *http.Request,
	) {
		var body chatRequest
		if err := json.NewDecoder(request.Body).Decode(&body); err != nil {
			t.Errorf("decode body: %v", err)
		}
		calls.Add(1)
		writer.Header().Set("Content-Type", "application/json")
		reply, err := json.Marshal(firstPersonAnswer)
		if err != nil {
			t.Fatalf("marshal reply: %v", err)
		}
		_, _ = writer.Write([]byte(
			`{"choices":[{"message":{"content":` + string(reply) + `}}]}`))
	}))
}

// exhaustRepair drives a neutral turn to repair exhaustion and returns the
// error, so each assertion below reads one property of the same failure.
func exhaustRepair(t *testing.T) error {
	t.Helper()
	var calls atomic.Int32
	server := refusingServer(t, &calls)
	defer server.Close()

	client := ProxyClient{
		BaseURL:       server.URL,
		Model:         "model",
		AuditRole:     "community",
		Attribution:   "Sirens Echo",
		ResponseStyle: ResponseStyleNeutral,
		HTTPClient:    &http.Client{Timeout: 2 * time.Second},
	}
	_, err := client.Complete(
		context.Background(),
		TurnPrompt{System: "system", Message: "do you have access to the aos general knowledge base?"},
		"request",
	)
	if err == nil {
		t.Fatal("a reply the neutral rule refuses was accepted")
	}
	// The premise: every call was answered. If the fixture ever fails a call
	// this test stops being about criterion 1.
	if calls.Load() < 2 {
		t.Fatalf("the repair path ran %d model calls, so it did not exhaust", calls.Load())
	}
	return err
}

func TestRepairExhaustionIsNotAModelFailure(t *testing.T) {
	t.Parallel()
	if err := exhaustRepair(t); !errors.Is(err, ErrResponseRepairExhausted) {
		t.Errorf("error does not carry ErrResponseRepairExhausted: %v", err)
	}
}

// The member-facing half, and the reason the issue was filed: they were told
// the backend was down while it answered every call in under eight seconds.
func TestTheMemberIsNotToldTheBackendIsDown(t *testing.T) {
	t.Parallel()
	notice := turnFailureNotice(stageModel, exhaustRepair(t))
	if notice == noticeModelFailed {
		t.Error("a turn whose every model call returned 200 reported an outage")
	}
	if notice != noticeReplyBlocked {
		t.Errorf("notice = %q, want the existing reply-blocked phrase %q",
			notice, noticeReplyBlocked)
	}
	// No new wording was authored for this path, which is deliberate.
	if !strings.Contains(notice, "response check") {
		t.Errorf("notice does not name the refusing layer: %q", notice)
	}
}

// The label and the phrase are documented as agreeing. A cause of stage_failed
// beside a reply-blocked notice would break that.
func TestTheLabelAgreesWithTheNotice(t *testing.T) {
	t.Parallel()
	err := exhaustRepair(t)
	if cause := failureCause(err); cause != causeReplyRefused {
		t.Errorf("failureCause = %q, want %q", cause, causeReplyRefused)
	}
}

// The refusing check stays in the error, because naming it is what made this
// issue diagnosable at all and the new wrapper must not displace it.
func TestTheRefusingCheckSurvivesTheWrapper(t *testing.T) {
	t.Parallel()
	err := exhaustRepair(t)
	if !strings.Contains(err.Error(), "first-person") {
		t.Errorf("the wrapped error no longer names what was refused: %v", err)
	}
}

// A real backend failure must still read as one, or this trades one wrong
// notice for another.
func TestAGenuineBackendFailureStillReportsAnOutage(t *testing.T) {
	t.Parallel()
	server := httptest.NewServer(http.HandlerFunc(func(
		writer http.ResponseWriter,
		_ *http.Request,
	) {
		http.Error(writer, "upstream is down", http.StatusServiceUnavailable)
	}))
	defer server.Close()

	client := ProxyClient{
		BaseURL:     server.URL,
		Model:       "model",
		AuditRole:   "community",
		Attribution: "Sirens Echo",
		HTTPClient:  &http.Client{Timeout: 2 * time.Second},
	}
	_, err := client.Complete(
		context.Background(),
		TurnPrompt{System: "system", Message: "user"},
		"request",
	)
	if err == nil {
		t.Fatal("a 503 from every attempt was treated as success")
	}
	if errors.Is(err, ErrResponseRepairExhausted) {
		t.Fatal("a backend outage was classified as a refused reply")
	}
	if notice := turnFailureNotice(stageModel, err); notice != noticeModelFailed {
		t.Errorf("notice = %q, want %q", notice, noticeModelFailed)
	}
}
