package community

import (
	"context"
	"sync/atomic"
)

// turnBudgetKey carries the allowance every completion under one turn shares.
type turnBudgetKey struct{}

// turnBudget is the model-call allowance one turn holds across the several
// completions it makes. See docs/sirens-echo-turn-stages.md.
type turnBudget struct {
	allowance atomic.Int64
}

// spend takes one model call and reports what is left after it. It goes
// negative rather than clamping, so an overspend stays visible to the caller.
func (b *turnBudget) spend() int {
	if b == nil {
		return -1
	}
	return int(b.allowance.Add(-1))
}

// affordsToolRound reports whether another tool round fits. One costs two
// calls, so a turn with a single call left can answer and cannot investigate.
func (b *turnBudget) affordsToolRound() bool {
	if b == nil {
		return true
	}
	return b.allowance.Load() > 1
}

// withTurnBudget installs an allowance for one turn. A non-positive allowance
// installs nothing, so a lane that has not opted in keeps today's behaviour.
func withTurnBudget(ctx context.Context, allowance int) context.Context {
	if allowance <= 0 {
		return ctx
	}
	budget := &turnBudget{}
	budget.allowance.Store(int64(allowance))
	return context.WithValue(ctx, turnBudgetKey{}, budget)
}

// turnBudgetFrom reports the allowance a context carries. A nil budget affords
// everything, which is what a caller outside a turn needs.
func turnBudgetFrom(ctx context.Context) *turnBudget {
	budget, _ := ctx.Value(turnBudgetKey{}).(*turnBudget)
	return budget
}
