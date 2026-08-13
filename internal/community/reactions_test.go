package community

import (
	"context"
	"errors"
	"testing"
)

type recordingReactor struct {
	applied []string
	// attempts counts what reached the transport, including what failed, which
	// is the cost the member never sees. See sirens-echo#460.
	attempts int
	err      error
}

func (r *recordingReactor) React(_ context.Context, emoji string) error {
	r.attempts++
	if r.err != nil {
		return r.err
	}
	r.applied = append(r.applied, emoji)
	return nil
}

// A reaction is a side effect on a path that already owes the member an answer,
// so a failing reaction must never surface.
func TestAFailingReactionIsSwallowed(t *testing.T) {
	t.Parallel()
	agent := &Agent{telemetry: telemetryOrNoop(nil)}
	failing := &recordingReactor{err: errors.New("missing ADD_REACTIONS")}
	agent.react(context.Background(), failing, reactionAccepted)
	// Reaching here without a panic or a returned error is the assertion.
	agent.react(context.Background(), nil, reactionAccepted)
}

// The tool round sits behind the completion boundary and reaches the reaction
// through the turn context, the same route the progress line takes.
func TestReactFromContextMarksTheTurn(t *testing.T) {
	t.Parallel()
	target := &recordingReactor{}
	ctx := WithReactor(context.Background(), target)
	reactFromContext(ctx, reactionTool)
	if len(target.applied) != 1 || target.applied[0] != reactionTool {
		t.Fatalf("applied = %v, want one tool reaction", target.applied)
	}
}

// Discord dedupes the visible reaction, so a repeat costs a request for a mark
// that is already there. A turn calling ten tools pays for one.
func TestATurnMarksTheMessageOncePerReaction(t *testing.T) {
	t.Parallel()
	target := &recordingReactor{}
	ctx := WithReactor(context.Background(), target)
	for range 10 {
		reactFromContext(ctx, reactionTool)
	}
	if target.attempts != 1 {
		t.Errorf("reached the transport %d times, want 1", target.attempts)
	}
	if len(target.applied) != 1 {
		t.Errorf("applied = %v, want one tool reaction", target.applied)
	}
}

// A reaction the bot has no permission for fails every time it is tried, so
// the repeat guard has to hold before the attempt rather than after it.
func TestAFailingReactionIsAttemptedOnce(t *testing.T) {
	t.Parallel()
	target := &recordingReactor{err: errors.New("missing ADD_REACTIONS")}
	ctx := WithReactor(context.Background(), target)
	for range 10 {
		reactFromContext(ctx, reactionTool)
	}
	if target.attempts != 1 {
		t.Errorf("retried a failing reaction %d times, want 1", target.attempts)
	}
}

// Once per reaction, not once per turn. A turn that calls a tool and then fails
// still carries both marks.
func TestADifferentReactionStillReachesTheMessage(t *testing.T) {
	t.Parallel()
	target := &recordingReactor{}
	ctx := WithReactor(context.Background(), target)
	reactFromContext(ctx, reactionTool)
	reactFromContext(ctx, reactionFailed)
	if len(target.applied) != 2 {
		t.Errorf("applied = %v, want both marks", target.applied)
	}
}

// The agent marks the turn directly and the tool loop marks it through the
// context. One applied set covers both, or the guard only half holds.
func TestTheAgentAndTheToolLoopShareOneAppliedSet(t *testing.T) {
	t.Parallel()
	agent := &Agent{telemetry: telemetryOrNoop(nil)}
	target := &recordingReactor{}
	ctx := WithReactor(context.Background(), target)
	agent.react(ctx, target, reactionFailed)
	reactFromContext(ctx, reactionFailed)
	agent.react(ctx, target, reactionFailed)
	if target.attempts != 1 {
		t.Errorf("reached the transport %d times, want 1", target.attempts)
	}
}

// A transport with no reaction surface must be inert rather than panic.
func TestReactFromContextWithoutAReactorIsInert(t *testing.T) {
	t.Parallel()
	reactFromContext(context.Background(), reactionTool)
	if ctx := WithReactor(context.Background(), nil); ctx.Value(reactionKey{}) != nil {
		t.Fatal("a nil reactor was stored on the context")
	}
}

// Every case Kai listed has its own mark, and no two share one.
func TestEveryHarnessCaseHasADistinctReaction(t *testing.T) {
	t.Parallel()
	seen := make(map[string]string)
	for name, emoji := range map[string]string{
		"accepted": reactionAccepted,
		"tool":     reactionTool,
		"failed":   reactionFailed,
		"refused":  reactionRefused,
	} {
		if emoji == "" {
			t.Errorf("%s has no reaction", name)
		}
		if existing, clash := seen[emoji]; clash {
			t.Errorf("%s and %s share the reaction %q", name, existing, emoji)
		}
		seen[emoji] = name
	}
}

// The approved vocabulary, pinned to codepoints. Asserting only distinctness
// is how two of these drifted. See sirens-echo#111.
func TestTheReactionsAreTheOnesThatWereApproved(t *testing.T) {
	t.Parallel()
	for name, want := range map[string]struct{ got, approved string }{
		"acknowledged":    {reactionAccepted, "\U0001F440"},
		"tool call":       {reactionTool, "\U0001F528"},
		"error":           {reactionFailed, "❌"},
		"content blocked": {reactionRefused, "\U0001F6AB"},
	} {
		if want.got != want.approved {
			t.Errorf("%s renders %q, want the approved %q", name, want.got, want.approved)
		}
	}
}
