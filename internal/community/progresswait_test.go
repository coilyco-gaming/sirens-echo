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

// Each edit adds one line, the shape Kai drew: three stacked lines rather than
// one changing number. Beats and edits differ now that it backs off (#934).
func TestEachEditAddsALine(t *testing.T) {
	t.Parallel()
	progress, sink, advance := waitingTurn(t)

	for len(sink.edits) < 3 {
		advance(turnProgressEvery)
		progress.refresh(context.Background())
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

// The column is bounded, so a turn that hangs cannot grow without limit. The
// bound is on height, not on how long it reports. See sirens-echo#899.
func TestTheWaitColumnIsBoundedAndKeepsUpdating(t *testing.T) {
	t.Parallel()
	progress, sink, advance := waitingTurn(t)

	// Long enough that the column fills and then keeps going, with the backoff
	// in force. Beats rather than edits, which is the distinction #934 added.
	beats := 1 << (maxProgressWaitLines + 2)
	for i := 0; i < beats; i++ {
		advance(turnProgressEvery)
		progress.refresh(context.Background())
	}
	// It still edits. The indicator going quiet is what a member read as the
	// turn stalling around 130 seconds, and that must not come back. See #899.
	if len(sink.edits) < maxProgressWaitLines {
		t.Fatalf("edits = %d over %d beats, so the indicator went quiet",
			len(sink.edits), beats)
	}
	final := sink.edits[len(sink.edits)-1]
	if got := strings.Count(final, "still "); got != maxProgressWaitLines {
		t.Errorf("final body carries %d wait lines, want %d", got, maxProgressWaitLines)
	}
	// The last line advances in place, so the newest elapsed is the one shown.
	if !strings.Contains(final, "seconds") {
		t.Errorf("final body reports no elapsed time: %q", final)
	}
}

// The defect #934 measured: 12 Discord writes for one reply, because every beat
// edited. The narration now costs a handful over the same wall clock.
func TestTheWaitNarrationBacksOff(t *testing.T) {
	t.Parallel()
	progress, sink, advance := waitingTurn(t)

	// The turn from the incident record: roughly three minutes of waiting at a
	// sixteen second beat, which produced ten edits.
	beats := 11
	for i := 0; i < beats; i++ {
		advance(turnProgressEvery)
		progress.refresh(context.Background())
	}
	if len(sink.edits) >= beats {
		t.Fatalf("%d beats produced %d edits, so nothing backed off",
			beats, len(sink.edits))
	}
	if len(sink.edits) > 5 {
		t.Errorf("a three minute wait cost %d Discord writes, want a handful",
			len(sink.edits))
	}
	if len(sink.edits) == 0 {
		t.Fatal("a waiting turn narrated nothing at all")
	}
}

// A new stage is new information, so its narration starts over rather than
// inheriting a budget the previous stage spent.
func TestAStageChangeRestartsTheBackoff(t *testing.T) {
	t.Parallel()
	progress, sink, advance := waitingTurn(t)

	for i := 0; i < 8; i++ {
		advance(turnProgressEvery)
		progress.refresh(context.Background())
	}
	spent := len(sink.edits)

	progress.Stage(context.Background(), stagePhraseChecking)
	before := len(sink.edits)
	advance(turnProgressEvery)
	progress.refresh(context.Background())
	if len(sink.edits) == before {
		t.Fatalf("the first beat after a stage change was swallowed by the "+
			"previous stage's backoff, %d edits spent before it", spent)
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
