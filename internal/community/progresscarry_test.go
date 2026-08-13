package community

import (
	"context"
	"errors"
	"testing"
	"time"
)

// A notice that could not be sent used to end with the progress line deleted
// and nothing in its place. See sirens-echo#624.

// failingEditSink refuses the edit, which is the case where the member is left
// with the last stage rather than the notice.
type failingEditSink struct {
	recordingSink
	editErr error
}

func (s *failingEditSink) Edit(_ context.Context, _, notice string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.edits = append(s.edits, notice)
	return s.editErr
}

// narratingProgress is a turn that has run long enough to have posted a line,
// which is the only turn this can happen to.
func narratingProgress(t *testing.T, sink TurnProgressSink) *turnProgress {
	t.Helper()
	moment := time.Now()
	progress := newTurnProgress(sink, func() time.Time { return moment })
	moment = moment.Add(turnProgressAfter + time.Second)
	progress.Stage(context.Background(), stagePhraseThinking)
	return progress
}

// The headline. A carried line survives the tidy-up, because it is the only
// thing the member received.
func TestACarriedLineIsNotDeleted(t *testing.T) {
	t.Parallel()
	sink := &recordingSink{}
	progress := narratingProgress(t, sink)
	if posts, _, _ := sink.counts(); posts != 1 {
		t.Fatalf("the fixture posted %d lines, want 1", posts)
	}

	progress.Carry(context.Background(), noticeTurnFailed)
	progress.Finish(context.Background())

	if _, _, deletes := sink.counts(); deletes != 0 {
		t.Errorf("the carried line was deleted %d times", deletes)
	}
	if got := sink.lastNotice(); got != noticeTurnFailed {
		t.Errorf("the line reads %q, want the notice %q", got, noticeTurnFailed)
	}
}

// The ordinary turn is untouched. A notice that sends normally never carries,
// and its narration still goes away.
func TestALineThatWasNotCarriedIsStillDeleted(t *testing.T) {
	t.Parallel()
	sink := &recordingSink{}
	progress := narratingProgress(t, sink)

	progress.Finish(context.Background())

	if _, _, deletes := sink.counts(); deletes != 1 {
		t.Errorf("deletes = %d, want the narration removed", deletes)
	}
}

// The promise this was built on. When the edit also fails, the member keeps the
// stale stage, because a stale line beats nothing at all.
func TestALineThatCouldNotBeCarriedIsStillNotDeleted(t *testing.T) {
	t.Parallel()
	sink := &failingEditSink{editErr: errors.New("discord rejected the edit")}
	progress := narratingProgress(t, sink)

	progress.Carry(context.Background(), noticeTurnFailed)
	progress.Finish(context.Background())

	if _, _, deletes := sink.counts(); deletes != 0 {
		t.Errorf("a line that could not be updated was deleted %d times", deletes)
	}
}

// A turn too short to narrate has no line to carry, so the failure path must
// not invent one. That is every ordinary failure.
func TestATurnWithNoProgressLineCarriesNothing(t *testing.T) {
	t.Parallel()
	sink := &recordingSink{}
	moment := time.Now()
	progress := newTurnProgress(sink, func() time.Time { return moment })

	progress.Carry(context.Background(), noticeTurnFailed)

	if posts, edits, _ := sink.counts(); posts != 0 || edits != 0 {
		t.Errorf("posts = %d, edits = %d, want a silent short turn", posts, edits)
	}
}

// Carry runs on the failure path, which reaches it through the context rather
// than through a reference. A transport with no progress line is inert.
func TestCarryFromContextIsSafeWithoutAProgressLine(t *testing.T) {
	t.Parallel()
	carryFromContext(context.Background(), noticeTurnFailed)

	sink := &recordingSink{}
	progress := narratingProgress(t, sink)
	ctx := WithTurnProgress(context.Background(), progress)

	carryFromContext(ctx, noticeTimedOut)

	if got := sink.lastNotice(); got != noticeTimedOut {
		t.Errorf("the line reads %q, want %q", got, noticeTimedOut)
	}
}

// An empty notice would blank the line, which is the same dead air by another
// route.
func TestAnEmptyNoticeDoesNotClaimTheLine(t *testing.T) {
	t.Parallel()
	sink := &recordingSink{}
	progress := narratingProgress(t, sink)

	progress.Carry(context.Background(), "   ")
	progress.Finish(context.Background())

	if _, _, deletes := sink.counts(); deletes != 1 {
		t.Errorf("deletes = %d, want an empty notice to change nothing", deletes)
	}
}

// A failed turn whose notice could not be delivered ends with the member
// holding the notice, which is the whole point of the change.
func TestAFailedTurnWhoseNoticeCannotSendLeavesTheNoticeOnTheLine(t *testing.T) {
	t.Parallel()
	agent := failingAgent(errors.New("upstream 502"))
	sink := &recordingSink{}
	progress := narratingProgress(t, sink)
	ctx := WithTurnProgress(context.Background(), progress)
	turn := &undeliverableTurn{}

	_ = agent.failTurn(ctx, turn, stageModel, errors.New("upstream 502"))
	progress.Finish(ctx)

	if got := sink.lastNotice(); got != noticeModelFailed {
		t.Errorf("the line reads %q, want %q", got, noticeModelFailed)
	}
	if _, _, deletes := sink.counts(); deletes != 0 {
		t.Errorf("the line carrying the notice was deleted %d times", deletes)
	}
}

// undeliverableTurn is a transport that cannot deliver, which is the condition
// the carry exists for.
type undeliverableTurn struct{ httpTurn }

func (t *undeliverableTurn) Reply(context.Context, string) error {
	return errors.New("discord refused the send")
}
