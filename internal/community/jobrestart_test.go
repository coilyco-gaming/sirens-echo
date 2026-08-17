package community

import (
	"context"
	"testing"
)

// #824 pinned that a restart drops queued work. This is the other half: the
// drop is now recorded and announced rather than silent. See sirens-echo#878.
func TestARestartSettlesAndAnnouncesWhatWasQueued(t *testing.T) {
	t.Parallel()
	store := NewMemoryJobStore(nil)
	queued := submitTestJob(t, store)
	notifier := newRecordingNotifier()
	agent := &Agent{
		jobs:      &JobRunner{Store: store, Notifier: notifier},
		telemetry: telemetryOrNoop(nil),
	}

	agent.recoverJobs(context.Background())

	settled, err := store.Get(queued.ID)
	if err != nil {
		t.Fatalf("Get: %v", err)
	}
	if settled.State != JobFailed {
		t.Errorf("a dropped job settled to %s, want %s", settled.State, JobFailed)
	}
	if settled.Outcome != "dropped by a restart" {
		t.Errorf("outcome = %q", settled.Outcome)
	}
	told := notifier.waitForOne(t)
	if told.ID != queued.ID {
		t.Errorf("notified about %s, want %s", told.ID, queued.ID)
	}
}

// A stranded job settled silently until now, which is the same defect one
// state over.
func TestARestartAnnouncesWhatItFoundRunning(t *testing.T) {
	t.Parallel()
	store := NewMemoryJobStore(nil)
	job := submitTestJob(t, store)
	if _, err := store.Transition(job.ID, JobRunning, nil); err != nil {
		t.Fatalf("to running: %v", err)
	}
	notifier := newRecordingNotifier()
	agent := &Agent{
		jobs:      &JobRunner{Store: store, Notifier: notifier},
		telemetry: telemetryOrNoop(nil),
	}

	agent.recoverJobs(context.Background())

	settled, err := store.Get(job.ID)
	if err != nil {
		t.Fatalf("Get: %v", err)
	}
	if settled.Outcome != "interrupted by a restart" {
		t.Errorf("outcome = %q", settled.Outcome)
	}
	if told := notifier.waitForOne(t); told.ID != job.ID {
		t.Errorf("notified about %s, want %s", told.ID, job.ID)
	}
}

// Recovery must not disturb a job that already reached a terminal state, or a
// restart would rewrite finished history.
func TestRestartRecoveryLeavesTerminalJobsAlone(t *testing.T) {
	t.Parallel()
	store := NewMemoryJobStore(nil)
	job := submitTestJob(t, store)
	if _, err := store.Transition(job.ID, JobRunning, nil); err != nil {
		t.Fatalf("to running: %v", err)
	}
	finished, err := store.Transition(job.ID, JobSucceeded, func(target *Job) {
		target.Outcome = "done"
	})
	if err != nil {
		t.Fatalf("to succeeded: %v", err)
	}
	notifier := newRecordingNotifier()
	agent := &Agent{
		jobs:      &JobRunner{Store: store, Notifier: notifier},
		telemetry: telemetryOrNoop(nil),
	}

	agent.recoverJobs(context.Background())

	current, err := store.Get(job.ID)
	if err != nil {
		t.Fatalf("Get: %v", err)
	}
	if current.State != finished.State || current.Outcome != finished.Outcome {
		t.Errorf("recovery rewrote a finished job: %s %q", current.State, current.Outcome)
	}
	notifier.mu.Lock()
	defer notifier.mu.Unlock()
	if len(notifier.jobs) != 0 {
		t.Errorf("recovery announced %d finished jobs", len(notifier.jobs))
	}
}

// The two groups are listed separately because they settle to different states
// under different phrases.
func TestDroppedAndStrandedJobsAreListedApart(t *testing.T) {
	t.Parallel()
	store := NewMemoryJobStore(nil)
	queued := submitTestJob(t, store)
	running, _, err := store.Submit(mustPrepare(t, "other-message"))
	if err != nil {
		t.Fatalf("Submit: %v", err)
	}
	if _, err := store.Transition(running.ID, JobRunning, nil); err != nil {
		t.Fatalf("to running: %v", err)
	}

	all := store.All()
	dropped := DroppedJobIDs(all)
	if len(dropped) != 1 || dropped[0] != queued.ID {
		t.Errorf("dropped = %v, want just %s", dropped, queued.ID)
	}
	stranded := StrandedJobIDs(all)
	if len(stranded) != 1 || stranded[0] != running.ID {
		t.Errorf("stranded = %v, want just %s", stranded, running.ID)
	}
}

func mustPrepare(t *testing.T, messageID string) Job {
	t.Helper()
	submission := discordSubmission()
	submission.Origin.MessageID = messageID
	prepared, err := PrepareJob(submission)
	if err != nil {
		t.Fatalf("PrepareJob: %v", err)
	}
	return prepared
}
