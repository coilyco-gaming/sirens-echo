package community

import (
	"strings"
	"testing"
	"time"
)

// Attribution answers who asked, from the record, without inference. See #151.
func TestEveryJobIsAttributableToItsRequester(t *testing.T) {
	t.Parallel()
	store := NewMemoryJobStore(fixedClock(time.Unix(1700000000, 0).UTC()))
	job := submitTestJob(t, store)

	principal, err := AttributeJob(store, job.ID)
	if err != nil {
		t.Fatalf("AttributeJob: %v", err)
	}
	if principal != job.Principal {
		t.Errorf("attributed to %q, want %q", principal, job.Principal)
	}
}

// Attribution has to survive the job, or a failure is the case nobody can
// trace and that is the case worth tracing.
func TestAttributionSurvivesEveryTerminalState(t *testing.T) {
	t.Parallel()
	for _, terminal := range []JobState{JobSucceeded, JobFailed, JobCancelled} {
		store := NewMemoryJobStore(fixedClock(time.Unix(1700000000, 0).UTC()))
		job := submitTestJob(t, store)
		if terminal != JobCancelled {
			if _, err := store.Transition(job.ID, JobRunning, nil); err != nil {
				t.Fatalf("to running: %v", err)
			}
		}
		if _, err := store.Transition(job.ID, terminal, nil); err != nil {
			t.Fatalf("to %s: %v", terminal, err)
		}
		principal, err := AttributeJob(store, job.ID)
		if err != nil {
			t.Fatalf("%s: AttributeJob: %v", terminal, err)
		}
		if principal != job.Principal {
			t.Errorf("%s: attributed to %q", terminal, principal)
		}
		listed, err := store.ListByPrincipal(job.Principal)
		if err != nil {
			t.Fatalf("ListByPrincipal: %v", err)
		}
		if len(listed) != 1 || listed[0].State != terminal {
			t.Errorf("%s: listing = %#v", terminal, listed)
		}
	}
}

// An effect on an external system traces back to the principal that caused it.
func TestAnExternalEffectTracesToItsPrincipal(t *testing.T) {
	t.Parallel()
	store := NewMemoryJobStore(fixedClock(time.Unix(1700000000, 0).UTC()))
	job := submitTestJob(t, store)
	if _, err := store.Transition(job.ID, JobRunning, nil); err != nil {
		t.Fatalf("to running: %v", err)
	}
	if _, err := RecordEffect(store, job.ID, "forgejo-issue", "142"); err != nil {
		t.Fatalf("RecordEffect: %v", err)
	}
	if _, err := store.Transition(job.ID, JobSucceeded, nil); err != nil {
		t.Fatalf("to succeeded: %v", err)
	}

	records, err := AttributeEffects(store, job.ID)
	if err != nil {
		t.Fatalf("AttributeEffects: %v", err)
	}
	if len(records) != 1 {
		t.Fatalf("records = %#v", records)
	}
	record := records[0]
	if record.Principal != job.Principal || record.Step != "forgejo-issue" || record.Detail != "142" {
		t.Errorf("record = %#v", record)
	}
	if record.State != JobSucceeded {
		t.Errorf("record state = %s", record.State)
	}
}

// The join is the record, so no user identifier needs to live in telemetry.
// That is design question two, answered by indirection.
func TestTelemetryCarriesTheJobIDAndNotThePrincipal(t *testing.T) {
	t.Parallel()
	telemetry, recorder, logs := jobTelemetry(t)
	job := Job{
		ID:        "job-abcdef0123",
		Kind:      "echo",
		Principal: "318190481467244544",
		Origin:    JobOrigin{Transport: transportDiscord},
	}
	ctx, span := telemetry.StartJobSpan(t.Context(), job)
	telemetry.Info(ctx, "job.step")
	span.End()

	for _, recorded := range recorder.Ended() {
		for _, attribute := range recorded.Attributes() {
			if strings.Contains(attribute.Value.AsString(), job.Principal) {
				t.Errorf("span %s carries the principal", recorded.Name())
			}
		}
	}
	if strings.Contains(logs.String(), job.Principal) {
		t.Errorf("a log row carries the principal: %s", logs.String())
	}
	if !strings.Contains(logs.String(), job.ID) {
		t.Error("a log row carries no job id, so nothing resolves to a principal")
	}
}

func TestAttributionReportsAMissingJob(t *testing.T) {
	t.Parallel()
	store := NewMemoryJobStore(nil)
	if _, err := AttributeJob(store, "job-does-not-exist"); err == nil {
		t.Error("attributed a job that does not exist")
	}
	if _, err := AttributeEffects(store, "job-does-not-exist"); err == nil {
		t.Error("attributed effects of a job that does not exist")
	}
}
