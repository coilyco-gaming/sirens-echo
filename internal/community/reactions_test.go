package community

import (
	"context"
	"errors"
	"testing"
)

type recordingReactor struct {
	applied []string
	err     error
}

func (r *recordingReactor) React(_ context.Context, emoji string) error {
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
