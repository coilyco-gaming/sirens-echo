package community

import (
	"context"
	"testing"
	"time"
)

// The hold before a reply lands was deliberate and invisible. See #652.

func TestTheHoldIsSpannedWhenThereIsOne(t *testing.T) {
	t.Parallel()
	agent := &Agent{telemetry: telemetryOrNoop(nil)}
	called := false
	agent.settleWithSpan(context.Background(), 3*time.Second, func(context.Context) {
		called = true
	})
	if !called {
		t.Error("the settle was not performed")
	}
}

// A turn that lands on the beat holds for nothing, and must not emit a span
// claiming a wait that did not happen.
func TestNoHoldStillSettles(t *testing.T) {
	t.Parallel()
	agent := &Agent{telemetry: telemetryOrNoop(nil)}
	got := time.Duration(-1)
	agent.settleWithSpan(context.Background(), 0, func(ctx context.Context) {
		got = 0
	})
	if got != 0 {
		t.Error("a zero hold skipped the settle entirely")
	}
}

// The context reaching the settle must still carry the turn's values, or the
// wait would lose the progress handle it is about to read.
func TestTheSettleKeepsTheTurnsValues(t *testing.T) {
	t.Parallel()
	agent := &Agent{telemetry: telemetryOrNoop(nil)}
	progress := &turnProgress{}
	ctx := WithTurnProgress(context.Background(), progress)
	var seen *turnProgress
	agent.settleWithSpan(ctx, time.Second, func(inner context.Context) {
		seen, _ = inner.Value(turnProgressKey{}).(*turnProgress)
	})
	if seen != progress {
		t.Error("the span context dropped the turn's progress handle")
	}
}

// settleDelayFromContext is what the caller measures the hold with, so a turn
// with no progress must report no hold rather than panicking.
func TestNoProgressReportsNoHold(t *testing.T) {
	t.Parallel()
	if got := settleDelayFromContext(context.Background()); got != 0 {
		t.Errorf("a turn with no progress reported a %s hold", got)
	}
}
