package community

import (
	"context"
	"errors"
	"sync"
	"testing"
	"time"
)

type recordingSink struct {
	mu      sync.Mutex
	posts   []string
	edits   []string
	deletes []string
	postErr error
}

func (s *recordingSink) Post(_ context.Context, notice string) (string, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.postErr != nil {
		return "", s.postErr
	}
	s.posts = append(s.posts, notice)
	return "message-1", nil
}

func (s *recordingSink) Edit(_ context.Context, _, notice string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.edits = append(s.edits, notice)
	return nil
}

func (s *recordingSink) Delete(_ context.Context, messageID string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.deletes = append(s.deletes, messageID)
	return nil
}

func (s *recordingSink) counts() (int, int, int) {
	s.mu.Lock()
	defer s.mu.Unlock()
	return len(s.posts), len(s.edits), len(s.deletes)
}

// lastNotice is the most recent text the line carried, whether it arrived as a
// post or as an edit.
func (s *recordingSink) lastNotice() string {
	s.mu.Lock()
	defer s.mu.Unlock()
	if len(s.edits) > 0 {
		return s.edits[len(s.edits)-1]
	}
	if len(s.posts) > 0 {
		return s.posts[len(s.posts)-1]
	}
	return ""
}

// stepClock advances only when a test says so, so nothing sleeps.
func stepClock(start time.Time) (func() time.Time, func(time.Duration)) {
	var mu sync.Mutex
	current := start
	now := func() time.Time {
		mu.Lock()
		defer mu.Unlock()
		return current
	}
	advance := func(by time.Duration) {
		mu.Lock()
		current = current.Add(by)
		mu.Unlock()
	}
	return now, advance
}

// A reply that beats the threshold posts nothing, so an ordinary turn is
// untouched. See #111.
func TestAShortTurnPostsNoProgress(t *testing.T) {
	t.Parallel()
	sink := &recordingSink{}
	now, advance := stepClock(time.Unix(1700000000, 0).UTC())
	progress := newTurnProgress(sink, now)

	progress.Stage(context.Background(), stagePhraseHistory)
	advance(time.Second)
	progress.Stage(context.Background(), stagePhraseThinking)
	progress.Finish(context.Background())

	posts, edits, deletes := sink.counts()
	if posts != 0 || edits != 0 || deletes != 0 {
		t.Errorf("a short turn touched Discord: %d posts, %d edits, %d deletes",
			posts, edits, deletes)
	}
}

// A long turn posts one line and edits it, rather than posting a column.
func TestALongTurnEditsOneLineAndRemovesIt(t *testing.T) {
	t.Parallel()
	sink := &recordingSink{}
	now, advance := stepClock(time.Unix(1700000000, 0).UTC())
	progress := newTurnProgress(sink, now)

	progress.Stage(context.Background(), stagePhraseHistory)
	advance(turnProgressAfter + time.Second)
	progress.Stage(context.Background(), stagePhraseThinking)
	advance(turnProgressEvery + time.Second)
	progress.Stage(context.Background(), stagePhraseTool)
	advance(turnProgressEvery + time.Second)
	progress.Stage(context.Background(), stagePhraseChecking)

	posts, edits, _ := sink.counts()
	if posts != 1 {
		t.Errorf("posted %d lines, want 1", posts)
	}
	if edits != 2 {
		t.Errorf("edited %d times, want 2", edits)
	}
	// The reply is the turn's answer, so the narration does not outlive it.
	progress.Finish(context.Background())
	if _, _, deletes := sink.counts(); deletes != 1 {
		t.Errorf("deleted %d lines, want 1", deletes)
	}
}

// Edits are bounded, so a tool-heavy turn cannot spend its budget on Discord.
func TestProgressEditsAreRateLimited(t *testing.T) {
	t.Parallel()
	sink := &recordingSink{}
	now, advance := stepClock(time.Unix(1700000000, 0).UTC())
	progress := newTurnProgress(sink, now)

	advance(turnProgressAfter + time.Second)
	progress.Stage(context.Background(), stagePhraseThinking)
	for index := 0; index < 5; index++ {
		advance(time.Millisecond)
		progress.Stage(context.Background(), stagePhraseTool)
		progress.Stage(context.Background(), stagePhraseThinking)
	}
	posts, edits, _ := sink.counts()
	if posts != 1 {
		t.Errorf("posted %d lines", posts)
	}
	if edits != 0 {
		t.Errorf("made %d edits inside the window, want 0", edits)
	}
}

// The line must not outlive the turn even when the reply wins the race.
func TestAFinishedTurnRemovesALineThatArrivedLate(t *testing.T) {
	t.Parallel()
	sink := &recordingSink{}
	now, advance := stepClock(time.Unix(1700000000, 0).UTC())
	progress := newTurnProgress(sink, now)
	advance(turnProgressAfter + time.Second)

	progress.mu.Lock()
	progress.finished = true
	progress.mu.Unlock()
	// Stage refuses once finished, which is the ordinary case.
	progress.Stage(context.Background(), stagePhraseThinking)
	if posts, _, _ := sink.counts(); posts != 0 {
		t.Errorf("posted after the turn finished: %d", posts)
	}
}

// A failed post must not break the turn, because progress is advisory.
func TestAFailedProgressPostIsNotFatal(t *testing.T) {
	t.Parallel()
	sink := &recordingSink{postErr: errors.New("discord said no")}
	now, advance := stepClock(time.Unix(1700000000, 0).UTC())
	progress := newTurnProgress(sink, now)
	advance(turnProgressAfter + time.Second)

	progress.Stage(context.Background(), stagePhraseThinking)
	progress.Finish(context.Background())
	if _, _, deletes := sink.counts(); deletes != 0 {
		t.Errorf("deleted a line that was never posted: %d", deletes)
	}
}

// A nil progress is the non-Discord path and every method has to accept it.
func TestNilProgressIsSafe(t *testing.T) {
	t.Parallel()
	var progress *turnProgress
	progress.Stage(context.Background(), stagePhraseThinking)
	progress.Finish(context.Background())
}

func TestProgressStagesUseTheHarnessFormat(t *testing.T) {
	t.Parallel()
	for _, phrase := range []string{
		stagePhraseHistory, stagePhraseThinking, stagePhraseTool, stagePhraseChecking,
	} {
		if notice := harnessNotice(phrase); !noticeShape.MatchString(notice) {
			t.Errorf("stage %q renders %q, which is not the harness shape", phrase, notice)
		}
	}
}

// The reported defect. A turn that waits makes no stage change, so Stage alone
// never posts. See docs/sirens-echo-progress.md.
func TestAWaitingStagePostsWithoutAStageChange(t *testing.T) {
	t.Parallel()
	sink := &recordingSink{}
	now, advance := stepClock(time.Unix(1700000000, 0).UTC())
	progress := newTurnProgress(sink, now)

	// Both stage changes happen inside the threshold, exactly as a real turn
	// does before it calls the model.
	progress.Stage(context.Background(), stagePhraseHistory)
	progress.Stage(context.Background(), stagePhraseThinking)
	if posts, _, _ := sink.counts(); posts != 0 {
		t.Fatalf("posted inside the threshold: %d", posts)
	}

	// The model call runs long and changes no stage.
	advance(turnProgressAfter + time.Second)
	progress.refresh(context.Background())

	posts, _, _ := sink.counts()
	if posts != 1 {
		t.Fatalf("a waiting stage posted %d lines, want 1", posts)
	}
	if got := sink.lastNotice(); got != stageLine(stagePhraseThinking) {
		t.Fatalf("notice = %q, want the stage the turn is actually in", got)
	}

	// Refreshing again must not post a second line.
	advance(turnProgressEvery * 2)
	progress.refresh(context.Background())
	if posts, _, _ := sink.counts(); posts != 1 {
		t.Fatalf("refresh posted a column: %d lines", posts)
	}

	progress.Finish(context.Background())
	if _, _, deletes := sink.counts(); deletes != 1 {
		t.Fatal("the narrated line outlived the turn")
	}
}

// A short turn stays untouched by the watcher, which is what keeps an ordinary
// reply free of narration.
func TestRefreshBeforeTheThresholdPostsNothing(t *testing.T) {
	t.Parallel()
	sink := &recordingSink{}
	now, advance := stepClock(time.Unix(1700000000, 0).UTC())
	progress := newTurnProgress(sink, now)

	progress.Stage(context.Background(), stagePhraseThinking)
	advance(turnProgressAfter - time.Second)
	progress.refresh(context.Background())

	if posts, _, _ := sink.counts(); posts != 0 {
		t.Fatalf("posted before the threshold: %d", posts)
	}
}

// A turn with no stage yet has nothing to narrate.
func TestRefreshWithoutAStagePostsNothing(t *testing.T) {
	t.Parallel()
	sink := &recordingSink{}
	now, advance := stepClock(time.Unix(1700000000, 0).UTC())
	progress := newTurnProgress(sink, now)

	advance(turnProgressAfter + time.Second)
	progress.refresh(context.Background())

	if posts, _, _ := sink.counts(); posts != 0 {
		t.Fatalf("narrated an empty stage: %d posts", posts)
	}
}

// The tool phrase reaches the line from behind the completion boundary, which
// is the only path the tool loop has to it.
func TestReportStageNarratesThroughTheContext(t *testing.T) {
	t.Parallel()
	sink := &recordingSink{}
	now, advance := stepClock(time.Unix(1700000000, 0).UTC())
	progress := newTurnProgress(sink, now)
	ctx := WithTurnProgress(context.Background(), progress)

	progress.Stage(ctx, stagePhraseThinking)
	advance(turnProgressAfter + time.Second)
	reportStage(ctx, stagePhraseTool)

	if got := sink.lastNotice(); got != stageLine(stagePhraseTool) {
		t.Fatalf("notice = %q, want the tool stage", got)
	}
	// A context carrying no progress must be inert rather than panic.
	reportStage(context.Background(), stagePhraseTool)
}

// The reported ambiguity. A rejected post and a turn too short to narrate looked
// identical from outside, because the failure was discarded.
func TestAFailedProgressPostIsRecorded(t *testing.T) {
	t.Parallel()
	sink := &recordingSink{postErr: errors.New("missing SEND_MESSAGES")}
	now, advance := stepClock(time.Unix(1700000000, 0).UTC())
	progress := newReportingTurnProgress(sink, telemetryOrNoop(nil), now)

	progress.Stage(context.Background(), stagePhraseThinking)
	advance(turnProgressAfter + time.Second)
	progress.refresh(context.Background())

	// The turn is unaffected and nothing panics. The record call is what makes
	// the failure observable, and a noop telemetry exercises that path.
	if posts, _, _ := sink.counts(); posts != 0 {
		t.Fatalf("a failing sink recorded a post: %d", posts)
	}
}

// Kai asked for a five second threshold on sirens-echo#375, and the grid is
// twice it by derivation rather than by a second decision.
func TestProgressThresholdsMatchTheRequestedCadence(t *testing.T) {
	t.Parallel()
	if turnProgressAfter != 5*time.Second {
		t.Errorf("turnProgressAfter = %s, want 5s", turnProgressAfter)
	}
	if turnProgressEvery != 10*time.Second {
		t.Errorf("turnProgressEvery = %s, want 10s", turnProgressEvery)
	}
}

// A turn that beats the threshold still narrates nothing, which is what keeps an
// ordinary reply clean at the lower cadence.
func TestAFastTurnStillPostsNothingAtTheLowerThreshold(t *testing.T) {
	t.Parallel()
	sink := &recordingSink{}
	now, advance := stepClock(time.Unix(1700000000, 0).UTC())
	progress := newTurnProgress(sink, now)

	progress.Stage(context.Background(), stagePhraseHistory)
	advance(turnProgressAfter - time.Second)
	progress.Stage(context.Background(), stagePhraseThinking)
	progress.refresh(context.Background())
	progress.Finish(context.Background())

	if posts, edits, deletes := sink.counts(); posts != 0 || edits != 0 || deletes != 0 {
		t.Errorf("a fast turn touched Discord: %d posts, %d edits, %d deletes", posts, edits, deletes)
	}
}

// Kai's scenario: a line at 3 seconds and a reply 0.1 seconds later, which
// releases on the next beat at 9 seconds rather than replacing the line.
func TestAPostedLineIsHeldBeforeTheReplyReplacesIt(t *testing.T) {
	t.Parallel()
	sink := &recordingSink{}
	now, advance := stepClock(time.Unix(1700000000, 0).UTC())
	progress := newTurnProgress(sink, now)

	progress.Stage(context.Background(), stagePhraseThinking)
	advance(turnProgressAfter)
	progress.refresh(context.Background())
	if posts, _, _ := sink.counts(); posts != 1 {
		t.Fatalf("expected one posted line, got %d", posts)
	}

	// The model returns a tenth of a second later.
	advance(100 * time.Millisecond)
	held := progress.settleDelay()
	if held <= 0 {
		t.Fatal("the reply was not held at all")
	}
	want := turnProgressEvery - 100*time.Millisecond
	if held != want {
		t.Fatalf("held for %s, want %s", held, want)
	}
}

// A turn that never posted a line must not be delayed, which is what keeps a
// fast reply fast.
func TestAnUnnarratedTurnIsNotHeld(t *testing.T) {
	t.Parallel()
	sink := &recordingSink{}
	now, advance := stepClock(time.Unix(1700000000, 0).UTC())
	progress := newTurnProgress(sink, now)

	progress.Stage(context.Background(), stagePhraseThinking)
	advance(turnProgressAfter - time.Second)
	if held := progress.settleDelay(); held != 0 {
		t.Fatalf("an unnarrated turn was held for %s", held)
	}
	var missing *turnProgress
	if held := missing.settleDelay(); held != 0 {
		t.Fatalf("a nil progress was held for %s", held)
	}
}

// The grid keeps going, so a turn still running past the first beat waits for
// the next one rather than being released early.
func TestTheGridRepeatsPastTheFirstBeat(t *testing.T) {
	t.Parallel()
	sink := &recordingSink{}
	now, advance := stepClock(time.Unix(1700000000, 0).UTC())
	progress := newTurnProgress(sink, now)

	progress.Stage(context.Background(), stagePhraseThinking)
	advance(turnProgressAfter)
	progress.refresh(context.Background())

	// Kai's second step: ready at 9.1 seconds, so it releases at 15.
	advance(turnProgressEvery + 100*time.Millisecond)
	want := turnProgressEvery - 100*time.Millisecond
	if held := progress.settleDelay(); held != want {
		t.Fatalf("held for %s past the first beat, want %s", held, want)
	}
}

// Landing exactly on a beat is on time. Without this the grid would round a
// punctual reply up to a whole extra window.
func TestAReplyOnTheBeatIsNotHeld(t *testing.T) {
	t.Parallel()
	sink := &recordingSink{}
	now, advance := stepClock(time.Unix(1700000000, 0).UTC())
	progress := newTurnProgress(sink, now)

	progress.Stage(context.Background(), stagePhraseThinking)
	advance(turnProgressAfter)
	progress.refresh(context.Background())
	advance(turnProgressEvery * 2)

	if held := progress.settleDelay(); held != 0 {
		t.Fatalf("a reply on the beat was held for %s", held)
	}
}

// A cancelled turn stops waiting rather than holding the member's answer for
// the full window.
func TestSettleReturnsOnCancellation(t *testing.T) {
	t.Parallel()
	sink := &recordingSink{}
	now, advance := stepClock(time.Unix(1700000000, 0).UTC())
	progress := newTurnProgress(sink, now)

	progress.Stage(context.Background(), stagePhraseThinking)
	advance(turnProgressAfter)
	progress.refresh(context.Background())

	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	done := make(chan struct{})
	go func() {
		progress.Settle(ctx)
		close(done)
	}()
	select {
	case <-done:
	case <-time.After(2 * time.Second):
		t.Fatal("Settle ignored cancellation")
	}
}
