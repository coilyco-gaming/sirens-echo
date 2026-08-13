package community

import (
	"context"
	"testing"
	"time"
)

// A failure notice must not thread. Threading calls the model for a title,
// inside a 10s budget written for a send. See sirens-echo#619.

// longRunningProgress is a turn that has posted a progress line and run past
// the threading window, which is the state a failed turn is usually in.
func longRunningProgress() *turnProgress {
	progress := &turnProgress{
		sink:  discardProgressSink{},
		now:   time.Now,
		start: time.Now().Add(-2 * turnLongReplyAfter),
	}
	progress.postedAt = time.Now().Add(-turnLongReplyAfter)
	return progress
}

type discardProgressSink struct{}

func (discardProgressSink) Post(context.Context, string) (string, error) { return "m", nil }
func (discardProgressSink) Edit(context.Context, string, string) error   { return nil }
func (discardProgressSink) Delete(context.Context, string) error         { return nil }

func TestALongTurnWantsThreading(t *testing.T) {
	t.Parallel()
	// The control. Without it the test below passes for the wrong reason.
	ctx := WithTurnProgress(context.Background(), longRunningProgress())
	if !turnLongReply(ctx) {
		t.Fatal("the fixture does not reproduce a threading turn, so the next test proves nothing")
	}
}

func TestANoticeDoesNotThread(t *testing.T) {
	t.Parallel()
	ctx := WithTurnProgress(context.Background(), longRunningProgress())
	if turnLongReply(withoutThreading(ctx)) {
		t.Error("a notice inherited the turn's threading mark")
	}
}

// context.WithoutCancel preserves values, which is how the mark reached the
// notice in the first place. Pinned so the cause cannot quietly return.
func TestWithoutCancelStillCarriesTheMark(t *testing.T) {
	t.Parallel()
	ctx := WithTurnProgress(context.Background(), longRunningProgress())
	if !turnLongReply(context.WithoutCancel(ctx)) {
		t.Skip("WithoutCancel no longer carries values, so this defect's cause is gone")
	}
	if turnLongReply(withoutThreading(context.WithoutCancel(ctx))) {
		t.Error("the notice path still threads after WithoutCancel")
	}
}

// An ordinary turn is untouched: stripping applies to notices only.
func TestAnOrdinaryReplyKeepsItsThreadingMark(t *testing.T) {
	t.Parallel()
	ctx := WithTurnProgress(context.Background(), longRunningProgress())
	if !turnLongReply(ctx) {
		t.Error("the ordinary reply path lost its threading mark")
	}
}
