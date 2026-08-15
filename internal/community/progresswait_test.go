package community

import (
	"context"
	"strings"
	"testing"
	"time"
)

// A turn that waits says how long. Before this it posted one line and left it
// static for the rest of the turn. See sirens-echo#370.

// waitSink records every edit, so a row can assert what a member would see
// rather than that no error came back.
type waitSink struct {
	posted string
	edits  []string
}

func (s *waitSink) Post(_ context.Context, notice string) (string, error) {
	s.posted = notice
	return "message-1", nil
}

func (s *waitSink) Edit(_ context.Context, _, notice string) error {
	s.edits = append(s.edits, notice)
	return nil
}

func (s *waitSink) Delete(context.Context, string) error { return nil }

// waitingTurn drives a progress past the posting threshold and returns a clock
// the caller advances, so every row controls its own elapsed time.
func waitingTurn(t *testing.T) (*turnProgress, *waitSink, func(time.Duration)) {
	t.Helper()
	moment := time.Now()
	sink := &waitSink{}
	progress := newTurnProgress(sink, func() time.Time { return moment })
	advance := func(by time.Duration) { moment = moment.Add(by) }

	progress.Stage(context.Background(), stagePhraseThinking)
	advance(turnProgressAfter + time.Second)
	progress.refresh(context.Background())
	if sink.posted == "" {
		t.Fatal("the fixture never posted a line, so nothing below is about waiting")
	}
	return progress, sink, advance
}

func TestAWaitingTurnSaysHowLong(t *testing.T) {
	t.Parallel()
	progress, sink, advance := waitingTurn(t)

	advance(turnProgressEvery)
	progress.refresh(context.Background())
	if len(sink.edits) != 1 {
		t.Fatalf("a beat after posting produced %d edits, want 1", len(sink.edits))
	}
	body := sink.edits[0]
	// The stage line survives above the wait line, which is what makes it a
	// column rather than a replacement.
	if !strings.Contains(body, stagePhraseThinking+"...") {
		t.Errorf("the stage line was lost: %q", body)
	}
	if !strings.Contains(body, "still "+stagePhraseThinking) {
		t.Errorf("no wait line was appended: %q", body)
	}
	if !strings.Contains(body, progressWaitIcon(0)) {
		t.Errorf("the wait line carries no clock: %q", body)
	}
}

// The hour hand advances as the lines pile up, which is what Kai asked for
// once she turned "if we wanna" into a decision. See sirens-echo#370.
func TestTheClockAdvancesDownTheColumn(t *testing.T) {
	t.Parallel()
	lines := strings.Split(progressBody(stagePhraseThinking, []int{9, 19, 29}), "\n")

	if len(lines) != 4 {
		t.Fatalf("body has %d lines, want the stage line and three waits", len(lines))
	}
	for index, want := range []string{"\U0001F550", "\U0001F551", "\U0001F552"} {
		if !strings.Contains(lines[index+1], want) {
			t.Errorf("wait line %d = %q, want the %d o'clock face",
				index, lines[index+1], index+1)
		}
	}
}

// Twelve faces against a cap of twelve lines means the wrap never fires today.
// It is here so the two can move independently of each other.
func TestTheRotationWraps(t *testing.T) {
	t.Parallel()
	if got := progressWaitIcon(len(progressWaitIcons)); got != progressWaitIcons[0] {
		t.Errorf("icon after a full turn = %q, want %q", got, progressWaitIcons[0])
	}
	if len(progressWaitIcons) != 12 {
		t.Errorf("the rotation has %d faces, want twelve", len(progressWaitIcons))
	}
}

// Only thinking and the clock lines are terminated, which is Kai's answer to
// the ellipsis having been applied to all four stages.
func TestOnlyThinkingAndTheClockAreTerminated(t *testing.T) {
	t.Parallel()
	for _, line := range strings.Split(progressBody(stagePhraseChecking, []int{9}), "\n") {
		terminated := strings.Contains(line, "...")
		clock := strings.Contains(line, "still ")
		if terminated != clock {
			t.Errorf("line %q: terminated=%v, want that only on the clock line",
				line, terminated)
		}
	}
}

// Each beat adds one line rather than replacing the last, which is the shape
// Kai drew: three stacked lines, not one changing number.
func TestEachBeatAddsALine(t *testing.T) {
	t.Parallel()
	progress, sink, advance := waitingTurn(t)

	for i := 0; i < 3; i++ {
		advance(turnProgressEvery)
		progress.refresh(context.Background())
	}
	if len(sink.edits) != 3 {
		t.Fatalf("three beats produced %d edits, want 3", len(sink.edits))
	}
	last := sink.edits[2]
	if got := strings.Count(last, "still "); got != 3 {
		t.Errorf("the final body carries %d wait lines, want 3: %q", got, last)
	}
}

// A beat that arrives early adds nothing, or a fast ticker turns the line into
// a wall regardless of the beat.
func TestAnEarlyBeatAddsNothing(t *testing.T) {
	t.Parallel()
	progress, sink, advance := waitingTurn(t)

	advance(turnProgressEvery / 2)
	progress.refresh(context.Background())
	if len(sink.edits) != 0 {
		t.Errorf("a beat inside the window edited: %v", sink.edits)
	}
}

// The column is bounded, so a turn that hangs cannot grow without limit.
func TestTheWaitColumnIsBounded(t *testing.T) {
	t.Parallel()
	progress, sink, advance := waitingTurn(t)

	for i := 0; i < maxProgressWaitLines+5; i++ {
		advance(turnProgressEvery)
		progress.refresh(context.Background())
	}
	if len(sink.edits) != maxProgressWaitLines {
		t.Fatalf("edits = %d, want the bound %d", len(sink.edits), maxProgressWaitLines)
	}
	if got := strings.Count(sink.edits[len(sink.edits)-1], "still "); got != maxProgressWaitLines {
		t.Errorf("final body carries %d wait lines, want %d", got, maxProgressWaitLines)
	}
}

// A stage change restarts the narration, because the new line is describing
// something else and its waits have not happened yet.
func TestAStageChangeClearsTheWaits(t *testing.T) {
	t.Parallel()
	progress, sink, advance := waitingTurn(t)

	advance(turnProgressEvery)
	progress.refresh(context.Background())
	advance(turnProgressEvery)
	progress.Stage(context.Background(), stagePhraseChecking)

	last := sink.edits[len(sink.edits)-1]
	if strings.Contains(last, "still ") {
		t.Errorf("the new stage inherited the previous stage's waits: %q", last)
	}
	if !strings.Contains(last, stagePhraseChecking) {
		t.Errorf("the new stage was not rendered: %q", last)
	}
}

// Every rendered line still passes the shape that lets a member tell a harness
// line from model output. Widening that is a security change, not formatting.
func TestEveryWaitLineKeepsTheNoticeShape(t *testing.T) {
	t.Parallel()
	body := progressBody(stagePhraseThinking, []int{9, 99, 900})
	for _, line := range strings.Split(body, "\n") {
		if !noticeShape.MatchString(line) {
			t.Errorf("line does not match noticeShape: %q", line)
		}
	}
}

// A finished turn narrates nothing, or a reply that already landed grows a
// line under it.
func TestAFinishedTurnStopsNarrating(t *testing.T) {
	t.Parallel()
	progress, sink, advance := waitingTurn(t)

	progress.Finish(context.Background())
	before := len(sink.edits)
	advance(turnProgressEvery)
	progress.refresh(context.Background())
	if len(sink.edits) != before {
		t.Errorf("a finished turn edited: %v", sink.edits[before:])
	}
}
