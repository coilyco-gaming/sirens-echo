package community

import (
	"errors"
	"path/filepath"
	"testing"
	"time"
)

func fixedClock(start time.Time) Clock {
	current := start
	return func() time.Time {
		current = current.Add(time.Second)
		return current
	}
}

func discordSubmission() Submission {
	return Submission{
		Kind:      "echo",
		Principal: "318190481467244544",
		Origin: JobOrigin{
			Transport: transportDiscord,
			ChannelID: "1537024102886277210",
			MessageID: "1537024279743434822",
		},
	}
}

func submitTestJob(t *testing.T, store JobStore) Job {
	t.Helper()
	prepared, err := PrepareJob(discordSubmission())
	if err != nil {
		t.Fatalf("PrepareJob: %v", err)
	}
	job, existed, err := store.Submit(prepared)
	if err != nil {
		t.Fatalf("Submit: %v", err)
	}
	if existed {
		t.Fatal("a first submission reported an existing job")
	}
	return job
}

// The state machine is the record's contract. An invalid move is an error
// rather than a silent overwrite. See #143.
func TestJobStateMachineEnumeratesItsTransitions(t *testing.T) {
	t.Parallel()
	allowed := []struct{ from, to JobState }{
		{JobQueued, JobRunning},
		{JobQueued, JobCancelled},
		{JobRunning, JobSucceeded},
		{JobRunning, JobFailed},
		{JobRunning, JobCancelling},
		{JobCancelling, JobCancelled},
		// A job that finishes while being cancelled is a real race, not a bug.
		{JobCancelling, JobSucceeded},
	}
	for _, move := range allowed {
		if !move.from.CanTransitionTo(move.to) {
			t.Errorf("%s to %s should be allowed", move.from, move.to)
		}
	}
	refused := []struct{ from, to JobState }{
		{JobSucceeded, JobRunning},
		{JobFailed, JobRunning},
		{JobCancelled, JobRunning},
		{JobSucceeded, JobFailed},
		{JobQueued, JobSucceeded},
	}
	for _, move := range refused {
		if move.from.CanTransitionTo(move.to) {
			t.Errorf("%s to %s should be refused", move.from, move.to)
		}
	}
	for _, state := range []JobState{JobSucceeded, JobFailed, JobCancelled} {
		if !state.Terminal() {
			t.Errorf("%s should be terminal", state)
		}
	}
	for _, state := range []JobState{JobQueued, JobRunning, JobCancelling} {
		if state.Terminal() {
			t.Errorf("%s should not be terminal", state)
		}
	}
}

func TestJobStoreRefusesAnInvalidTransition(t *testing.T) {
	t.Parallel()
	store := NewMemoryJobStore(fixedClock(time.Unix(1700000000, 0).UTC()))
	job := submitTestJob(t, store)

	if _, err := store.Transition(job.ID, JobSucceeded, nil); err == nil {
		t.Fatal("a queued job moved straight to succeeded")
	} else if !IsJobTransitionError(err) {
		t.Errorf("error is not a transition refusal: %v", err)
	}
	// The refusal must not have written anything.
	unchanged, err := store.Get(job.ID)
	if err != nil {
		t.Fatalf("Get: %v", err)
	}
	if unchanged.State != JobQueued {
		t.Errorf("state after a refused move = %s", unchanged.State)
	}
}

func TestJobStoreStampsTheLifecycle(t *testing.T) {
	t.Parallel()
	store := NewMemoryJobStore(fixedClock(time.Unix(1700000000, 0).UTC()))
	job := submitTestJob(t, store)
	if job.Attempts != 0 || !job.StartedAt.IsZero() || !job.EndedAt.IsZero() {
		t.Fatalf("a queued job is already stamped: %#v", job)
	}

	running, err := store.Transition(job.ID, JobRunning, nil)
	if err != nil {
		t.Fatalf("to running: %v", err)
	}
	if running.Attempts != 1 || running.StartedAt.IsZero() {
		t.Errorf("running job = %#v", running)
	}

	done, err := store.Transition(job.ID, JobSucceeded, func(target *Job) {
		target.Outcome = "echoed"
	})
	if err != nil {
		t.Fatalf("to succeeded: %v", err)
	}
	if done.EndedAt.IsZero() || done.Outcome != "echoed" {
		t.Errorf("finished job = %#v", done)
	}
	if !done.EndedAt.After(done.StartedAt) {
		t.Errorf("ended %v is not after started %v", done.EndedAt, done.StartedAt)
	}
}

// An at-least-once transport redelivers, so the harness owns dedup.
func TestSubmittingTheSameRequestTwiceYieldsOneJob(t *testing.T) {
	t.Parallel()
	store := NewMemoryJobStore(fixedClock(time.Unix(1700000000, 0).UTC()))
	first := submitTestJob(t, store)

	prepared, err := PrepareJob(discordSubmission())
	if err != nil {
		t.Fatalf("PrepareJob: %v", err)
	}
	second, existed, err := store.Submit(prepared)
	if err != nil {
		t.Fatalf("resubmit: %v", err)
	}
	if !existed {
		t.Error("a redelivered submission did not report the existing job")
	}
	if second.ID != first.ID {
		t.Errorf("redelivery made a second job: %s and %s", first.ID, second.ID)
	}
	if listed, err := store.ListByPrincipal(first.Principal); err != nil {
		t.Fatalf("ListByPrincipal: %v", err)
	} else if len(listed) != 1 {
		t.Errorf("principal has %d jobs, want 1", len(listed))
	}
}

// A different message is a different request even from the same principal.
func TestADifferentOriginIsADifferentJob(t *testing.T) {
	t.Parallel()
	store := NewMemoryJobStore(fixedClock(time.Unix(1700000000, 0).UTC()))
	first := submitTestJob(t, store)

	other := discordSubmission()
	other.Origin.MessageID = "1537024279743434823"
	prepared, err := PrepareJob(other)
	if err != nil {
		t.Fatalf("PrepareJob: %v", err)
	}
	second, existed, err := store.Submit(prepared)
	if err != nil {
		t.Fatalf("Submit: %v", err)
	}
	if existed || second.ID == first.ID {
		t.Errorf("two messages collapsed onto one job: %s", second.ID)
	}
}

func TestPrepareJobRejectsAnUnknownKindOrMissingPrincipal(t *testing.T) {
	t.Parallel()
	unknown := discordSubmission()
	unknown.Kind = "deploy-everything"
	if _, err := PrepareJob(unknown); err == nil {
		t.Error("accepted an undeclared job kind")
	}
	anonymous := discordSubmission()
	anonymous.Principal = ""
	if _, err := PrepareJob(anonymous); err == nil {
		t.Error("accepted a job with no requesting principal")
	}
}

func TestJobValidationRejectsAnUnanswerableDiscordOrigin(t *testing.T) {
	t.Parallel()
	job := Job{
		ID:             "job-0123456789",
		IdempotencyKey: "k",
		Principal:      "p",
		Kind:           "echo",
		State:          JobQueued,
		CreatedAt:      time.Unix(1700000000, 0).UTC(),
		Origin:         JobOrigin{Transport: transportDiscord},
	}
	if err := job.Validate(); err == nil {
		t.Error("accepted a Discord job with no channel to answer in")
	}
	job.Origin.Transport = "carrier-pigeon"
	if err := job.Validate(); err == nil {
		t.Error("accepted an unknown transport")
	}
}

// The record has to survive a restart with its state intact, which is the
// whole reason the store is durable.
func TestFileJobStoreSurvivesARestart(t *testing.T) {
	t.Parallel()
	dir := filepath.Join(t.TempDir(), "jobs")
	store, err := OpenFileJobStore(dir, fixedClock(time.Unix(1700000000, 0).UTC()))
	if err != nil {
		t.Fatalf("OpenFileJobStore: %v", err)
	}
	job := submitTestJob(t, store)
	if _, err := store.Transition(job.ID, JobRunning, nil); err != nil {
		t.Fatalf("to running: %v", err)
	}

	reopened, err := OpenFileJobStore(dir, fixedClock(time.Unix(1700001000, 0).UTC()))
	if err != nil {
		t.Fatalf("reopen: %v", err)
	}
	recovered, err := reopened.Get(job.ID)
	if err != nil {
		t.Fatalf("Get after restart: %v", err)
	}
	if recovered.State != JobRunning {
		t.Errorf("state after restart = %s, want running", recovered.State)
	}
	if recovered.Principal != job.Principal || recovered.Kind != job.Kind {
		t.Errorf("record changed across restart: %#v", recovered)
	}
	// Dedup has to survive too, or a redelivery after a restart makes a twin.
	prepared, err := PrepareJob(discordSubmission())
	if err != nil {
		t.Fatalf("PrepareJob: %v", err)
	}
	again, existed, err := reopened.Submit(prepared)
	if err != nil {
		t.Fatalf("resubmit after restart: %v", err)
	}
	if !existed || again.ID != job.ID {
		t.Errorf("redelivery after a restart made a second job: %#v", again)
	}
}

// A crash can only strand a job that was in flight. Queued is still accurate.
func TestRestartRecoveryOnlyMovesStrandedJobs(t *testing.T) {
	t.Parallel()
	store := NewMemoryJobStore(fixedClock(time.Unix(1700000000, 0).UTC()))
	running := submitTestJob(t, store)
	if _, err := store.Transition(running.ID, JobRunning, nil); err != nil {
		t.Fatalf("to running: %v", err)
	}
	queuedSubmission := discordSubmission()
	queuedSubmission.Origin.MessageID = "1537024279743434899"
	prepared, err := PrepareJob(queuedSubmission)
	if err != nil {
		t.Fatalf("PrepareJob: %v", err)
	}
	queued, _, err := store.Submit(prepared)
	if err != nil {
		t.Fatalf("Submit: %v", err)
	}

	stranded := StrandedJobIDs(store.All())
	if len(stranded) != 1 || stranded[0] != running.ID {
		t.Fatalf("stranded = %v, want just %s", stranded, running.ID)
	}
	recovered, err := RecoverStrandedJobs(store, stranded, "interrupted by a restart")
	if err != nil {
		t.Fatalf("RecoverStrandedJobs: %v", err)
	}
	if len(recovered) != 1 || recovered[0].State != JobFailed {
		t.Fatalf("recovered = %#v", recovered)
	}
	untouched, err := store.Get(queued.ID)
	if err != nil {
		t.Fatalf("Get: %v", err)
	}
	if untouched.State != JobQueued {
		t.Errorf("a queued job was moved by recovery: %s", untouched.State)
	}
}

// A resumed job must not double-apply what it already did.
func TestEffectsAreRecordedOnceAndSurviveTransitions(t *testing.T) {
	t.Parallel()
	store := NewMemoryJobStore(fixedClock(time.Unix(1700000000, 0).UTC()))
	job := submitTestJob(t, store)
	if _, err := store.Transition(job.ID, JobRunning, nil); err != nil {
		t.Fatalf("to running: %v", err)
	}

	first, err := RecordEffect(store, job.ID, "issue-filed", "42")
	if err != nil {
		t.Fatalf("RecordEffect: %v", err)
	}
	if !EffectApplied(first, "issue-filed") {
		t.Fatal("effect was not recorded")
	}
	second, err := RecordEffect(store, job.ID, "issue-filed", "43")
	if err != nil {
		t.Fatalf("second RecordEffect: %v", err)
	}
	if second.Effects["issue-filed"] != "42" {
		t.Errorf("a repeated effect overwrote the first: %v", second.Effects)
	}
	done, err := store.Transition(job.ID, JobSucceeded, nil)
	if err != nil {
		t.Fatalf("to succeeded: %v", err)
	}
	if !EffectApplied(done, "issue-filed") {
		t.Error("effects did not survive a transition")
	}
}

func TestJobStoreReportsAMissingJob(t *testing.T) {
	t.Parallel()
	store := NewMemoryJobStore(nil)
	if _, err := store.Get("job-does-not-exist"); !errors.Is(err, ErrJobNotFound) {
		t.Errorf("error for a missing job = %v", err)
	}
	if _, err := store.Transition("job-does-not-exist", JobRunning, nil); !errors.Is(err, ErrJobNotFound) {
		t.Errorf("transition error for a missing job = %v", err)
	}
}
