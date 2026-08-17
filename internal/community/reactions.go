package community

import (
	"context"
	"log/slog"
	"sync"
)

// A reaction is harness state on the member's own message, not model output. It
// never reaches reply validation. See docs/sirens-echo-progress.md.

const (
	// reactionAccepted marks a message the harness took, before any model call.
	// A turn that then dies silently is still visible as an unanswered mark.
	reactionAccepted = "\U0001F440"
	// reactionTool marks a turn that called a tool.
	reactionTool = "\U0001F528"
	// reactionFailed marks a turn that did not produce a reply.
	reactionFailed = "\u274C"
	// reactionRefused marks a message a boundary turned away.
	reactionRefused = "\U0001F6AB"
)

// reactor applies one reaction to the message that started a turn. A transport
// with no reaction surface implements nothing and is skipped.
type reactor interface {
	React(ctx context.Context, emoji string) error
}

// unreactor removes a mark the harness applied. A transport that can add a
// reaction and not remove one implements nothing here and keeps its marks.
type unreactor interface {
	Unreact(ctx context.Context, emoji string) error
}

// transientReactions describe work in flight rather than an outcome, so they
// stop being true when the turn ends. See docs/sirens-echo-progress.md.
var transientReactions = []string{reactionAccepted, reactionTool}

// onceReactor applies each emoji once for the turn it belongs to. A turn that
// calls ten tools marks the message once. See sirens-echo#460.
type onceReactor struct {
	inner   reactor
	mu      sync.Mutex
	applied map[string]bool
}

// React drops a repeat before it reaches the transport, marking before the
// attempt so a refused reaction is not retried. See docs/sirens-echo-progress.md.
func (o *onceReactor) React(ctx context.Context, emoji string) error {
	o.mu.Lock()
	repeat := o.applied[emoji]
	o.applied[emoji] = true
	o.mu.Unlock()
	if repeat {
		return nil
	}
	return o.inner.React(ctx, emoji)
}

// appliedTransient names the in-flight marks this turn actually put on the
// message, so a clear costs no call for a mark that was never applied.
func (o *onceReactor) appliedTransient() []string {
	o.mu.Lock()
	defer o.mu.Unlock()
	applied := make([]string, 0, len(transientReactions))
	for _, emoji := range transientReactions {
		if o.applied[emoji] {
			applied = append(applied, emoji)
		}
	}
	return applied
}

// reactionKey carries the turn's reactor to layers that hold no reference to
// the transport, the same route the progress line takes.
type reactionKey struct{}

// WithReactor marks a context as able to react to the message that started it.
func WithReactor(ctx context.Context, target reactor) context.Context {
	if target == nil {
		return ctx
	}
	return context.WithValue(ctx, reactionKey{}, &onceReactor{
		inner:   target,
		applied: make(map[string]bool, 2),
	})
}

// turnReactor prefers the context's reactor, so every mark on a turn shares one
// applied set rather than each call site keeping its own.
func turnReactor(ctx context.Context, target reactor) reactor {
	if carried, ok := ctx.Value(reactionKey{}).(reactor); ok {
		return carried
	}
	return target
}

// react applies a reaction and swallows every failure. A reaction is a side
// effect on a path that already owes the member an answer.
func (a *Agent) react(ctx context.Context, target reactor, emoji string) {
	if target == nil {
		return
	}
	target = turnReactor(ctx, target)
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

// clearTurnMarks removes the marks that described work in flight, leaving any
// mark that describes the outcome. See docs/sirens-echo-progress.md.
func (a *Agent) clearTurnMarks(ctx context.Context) {
	once, ok := ctx.Value(reactionKey{}).(*onceReactor)
	if !ok {
		return
	}
	remover, ok := once.inner.(unreactor)
	if !ok {
		return
	}
	applied := once.appliedTransient()
	if len(applied) == 0 {
		return
	}
	clearCtx, cancel := context.WithTimeout(
		context.WithoutCancel(ctx),
		reactionClearTimeout,
	)
	defer cancel()
	for _, emoji := range applied {
		if err := remover.Unreact(clearCtx, emoji); err != nil {
			// Same posture as applying one. A tidy-up must never cost a member
			// an answer they already received.
			a.telemetry.Info(
				clearCtx,
				"discord.reaction.remove.failed",
				slog.String("reaction", emoji),
				slog.String("error", err.Error()),
			)
		}
	}
}
