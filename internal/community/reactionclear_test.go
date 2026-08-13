package community

import (
	"context"
	"errors"
	"testing"
)

// A mark that describes work in flight stops being true when the turn ends.
// See sirens-echo#475.

// clearingReactor records both directions, so a test can tell a mark that was
// never applied from one applied and then removed.
type clearingReactor struct {
	applied []string
	removed []string
	err     error
}

func (c *clearingReactor) React(_ context.Context, emoji string) error {
	c.applied = append(c.applied, emoji)
	return nil
}

func (c *clearingReactor) Unreact(_ context.Context, emoji string) error {
	if c.err != nil {
		return c.err
	}
	c.removed = append(c.removed, emoji)
	return nil
}

// addOnly can mark and not unmark, which is the transport contract a surface
// is allowed to implement half of.
type addOnly struct{ applied []string }

func (a *addOnly) React(_ context.Context, emoji string) error {
	a.applied = append(a.applied, emoji)
	return nil
}

func markedTurn(t *testing.T, target reactor, marks ...string) context.Context {
	t.Helper()
	agent := &Agent{telemetry: telemetryOrNoop(nil)}
	ctx := WithReactor(context.Background(), target)
	for _, emoji := range marks {
		agent.react(ctx, target, emoji)
	}
	return ctx
}

// The reported ask. The two in-flight marks go and the outcome mark stays.
func TestTheInFlightMarksAreClearedAndTheOutcomeMarkStays(t *testing.T) {
	t.Parallel()
	target := &clearingReactor{}
	ctx := markedTurn(t, target, reactionAccepted, reactionTool, reactionFailed)

	(&Agent{telemetry: telemetryOrNoop(nil)}).clearTurnMarks(ctx)

	if len(target.removed) != 2 {
		t.Fatalf("removed %v, want the eyes and the hammer", target.removed)
	}
	for _, emoji := range []string{reactionAccepted, reactionTool} {
		if !contains(target.removed, emoji) {
			t.Errorf("%q survived the turn it was describing", emoji)
		}
	}
	if contains(target.removed, reactionFailed) {
		t.Error("the failure mark was removed, so the outcome left no trace")
	}
}

// A boundary refusal is an outcome by the same reasoning, so it stays too.
func TestABoundaryRefusalIsNotCleared(t *testing.T) {
	t.Parallel()
	target := &clearingReactor{}
	ctx := markedTurn(t, target, reactionAccepted, reactionRefused)

	(&Agent{telemetry: telemetryOrNoop(nil)}).clearTurnMarks(ctx)

	if contains(target.removed, reactionRefused) {
		t.Error("the refusal mark was removed, so the boundary left no trace")
	}
	if !contains(target.removed, reactionAccepted) {
		t.Error("the accepted mark survived")
	}
}

// A turn that called no tool never applied the hammer, so removing it would be
// a Discord call for a reaction that was never there.
func TestOnlyTheMarksActuallyAppliedAreRemoved(t *testing.T) {
	t.Parallel()
	target := &clearingReactor{}
	ctx := markedTurn(t, target, reactionAccepted)

	(&Agent{telemetry: telemetryOrNoop(nil)}).clearTurnMarks(ctx)

	if len(target.removed) != 1 || target.removed[0] != reactionAccepted {
		t.Errorf("removed %v, want only the accepted mark", target.removed)
	}
}

// A turn that marked nothing costs nothing to clear.
func TestClearingAnUnmarkedTurnCallsTheTransportNotAtAll(t *testing.T) {
	t.Parallel()
	target := &clearingReactor{}
	ctx := WithReactor(context.Background(), target)

	(&Agent{telemetry: telemetryOrNoop(nil)}).clearTurnMarks(ctx)

	if len(target.removed) != 0 {
		t.Errorf("removed %v on a turn that marked nothing", target.removed)
	}
}

// A transport that can mark and not unmark keeps its marks rather than failing.
func TestATransportThatCannotRemoveIsInert(t *testing.T) {
	t.Parallel()
	target := &addOnly{}
	ctx := markedTurn(t, target, reactionAccepted, reactionTool)

	(&Agent{telemetry: telemetryOrNoop(nil)}).clearTurnMarks(ctx)
	// Reaching here without a panic is the assertion.
	if len(target.applied) != 2 {
		t.Errorf("applied = %v, want both marks", target.applied)
	}
}

// A turn with no reactor at all, which is every transport that is not Discord.
func TestClearingATurnWithNoReactorIsInert(t *testing.T) {
	t.Parallel()
	(&Agent{telemetry: telemetryOrNoop(nil)}).clearTurnMarks(context.Background())
}

// A tidy-up must never cost a member the answer they already received, so a
// failing removal is swallowed the way a failing application is.
func TestAFailingRemovalIsSwallowed(t *testing.T) {
	t.Parallel()
	target := &clearingReactor{err: errors.New("missing MANAGE_MESSAGES")}
	ctx := markedTurn(t, target, reactionAccepted)

	(&Agent{telemetry: telemetryOrNoop(nil)}).clearTurnMarks(ctx)
	// Reaching here is the assertion.
}

// The split itself. A mark is either about work in flight or about an outcome,
// and nothing may be both.
func TestNoOutcomeMarkIsAlsoTransient(t *testing.T) {
	t.Parallel()
	for _, emoji := range []string{reactionFailed, reactionRefused} {
		if contains(transientReactions, emoji) {
			t.Errorf("%q is an outcome and would be cleared", emoji)
		}
	}
	for _, emoji := range []string{reactionAccepted, reactionTool} {
		if !contains(transientReactions, emoji) {
			t.Errorf("%q describes work in flight and would survive it", emoji)
		}
	}
}

func contains(values []string, want string) bool {
	for _, value := range values {
		if value == want {
			return true
		}
	}
	return false
}
