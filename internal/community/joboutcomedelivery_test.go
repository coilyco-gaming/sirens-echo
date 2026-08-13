package community

import "testing"

// JobExecutor returns a string and the runner stores it on Job.Outcome. The
// terminal notice reads State and never Outcome. See sirens-echo#356.

// Two jobs differing only in what the executor produced. A member is told the
// same thing about both, which is the discard sirens-echo#356 is about.
func TestTheTerminalNoticeDiscardsWhatTheJobProduced(t *testing.T) {
	t.Parallel()
	found := Job{ID: "j1", State: JobSucceeded, Outcome: "17 stale entries, 3 repaired"}
	silent := Job{ID: "j1", State: JobSucceeded, Outcome: ""}

	if jobTerminalNotice(found) != jobTerminalNotice(silent) {
		t.Errorf("the terminal notice now varies with Outcome: %q against %q. "+
			"If sirens-echo#356 is being closed, this test is the wrong shape "+
			"and should assert what a member receives instead",
			jobTerminalNotice(found), jobTerminalNotice(silent))
	}
}

// The three phrases a job can end on, whatever it did. Pinned so a fourth or a
// rendered outcome is a deliberate change rather than a drift.
func TestAJobEndsOnOneOfThreePhrases(t *testing.T) {
	t.Parallel()
	for _, row := range []struct {
		state Job
		want  string
	}{
		{Job{ID: "j1", State: JobSucceeded, Outcome: "anything at all"}, "job j1 finished"},
		{Job{ID: "j1", State: JobCancelled, Outcome: "anything at all"}, "job j1 cancelled"},
		{Job{ID: "j1", State: JobFailed, Outcome: "anything at all"}, "job j1 failed"},
	} {
		if got := jobTerminalNotice(row.state); got != harnessNotice(row.want) {
			t.Errorf("terminal notice for %s = %q, want %q",
				row.state.State, got, harnessNotice(row.want))
		}
	}
}
