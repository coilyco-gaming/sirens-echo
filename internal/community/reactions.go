package community

import (
	"context"
	"log/slog"
)

// A reaction is harness state on the member's own message, not model output. It
// never reaches reply validation. See docs/sirens-echo-reactions.md.

const (
	// reactionAccepted marks a message the harness took, before any model call.
	// A turn that then dies silently is still visible as an unanswered mark.
	reactionAccepted = "\U0001F440"
	// reactionTool marks a turn that called a tool.
	reactionTool = "\U0001F528"
	// reactionFailed marks a turn that did not produce a reply.
	reactionFailed = "⚠️"
	// reactionRefused marks a message a boundary turned away.
	reactionRefused = "⛔"
)

// reactor applies one reaction to the message that started a turn. A transport
// with no reaction surface implements nothing and is skipped.
type reactor interface {
	React(ctx context.Context, emoji string) error
}

// reactionKey carries the turn's reactor to layers that hold no reference to
// the transport, the same route the progress line takes.
type reactionKey struct{}

// WithReactor marks a context as able to react to the message that started it.
func WithReactor(ctx context.Context, target reactor) context.Context {
	if target == nil {
		return ctx
	}
	return context.WithValue(ctx, reactionKey{}, target)
}

// react applies a reaction and swallows every failure. A reaction is a side
// effect on a path that already owes the member an answer.
func (a *Agent) react(ctx context.Context, target reactor, emoji string) {
	if target == nil {
		return
	}
	if err := target.React(ctx, emoji); err != nil {
		// Most likely a missing ADD_REACTIONS permission, which is an operator
		// question rather than a turn failure.
		a.telemetry.Info(
			ctx,
			"discord.reaction.failed",
			slog.String("reaction", emoji),
			slog.String("error", err.Error()),
		)
	}
}

// reactFromContext lets a layer behind the completion boundary mark the turn
// without taking a transport argument it has no other use for.
func reactFromContext(ctx context.Context, emoji string) {
	target, _ := ctx.Value(reactionKey{}).(reactor)
	if target == nil {
		return
	}
	// Nothing is logged here. This path has no telemetry handle, and a reaction
	// failure is already reported wherever the agent applies one.
	_ = target.React(ctx, emoji)
}
