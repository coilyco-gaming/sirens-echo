package community

import (
	"path/filepath"
	"testing"
)

// A ceiling a ladder cannot reach is decoration, and the deployment reasons
// about the ceiling. See sirens-echo#522 and deploy#431.

// ladderTop returns the largest completion budget a definition can actually
// reach, by climbing the same rungs the proxy climbs.
func ladderTop(budget ModelBudget) int {
	return budget.resolved().ladderTop()
}

// modelCallsBeforeBudgetSpent is the first call plus one per rung the ladder
// climbs, which is fewer than the allowed raises once the ceiling is reached.
func modelCallsBeforeBudgetSpent(budget ModelBudget) int {
	resolved := budget.resolved()
	calls, tokens := 1, resolved.BaseCompletionTokens
	for raise := 0; raise < resolved.BudgetRaises; raise++ {
		next, canRaise := nextCompletionBudget(tokens, resolved.MaxCompletionTokens)
		if !canRaise {
			break
		}
		tokens = next
		calls++
	}
	return calls
}

// Every shipped definition, so raising a ceiling without adding a rung fails
// here rather than granting headroom the runtime never uses.
func TestEveryDefinitionsLadderReachesItsCeiling(t *testing.T) {
	t.Parallel()
	definitions := trackedDefinitionPaths(t)
	for _, path := range definitions {
		definition, err := LoadDefinition(path)
		if err != nil {
			t.Fatalf("load %s: %v", path, err)
		}
		budget := definition.ModelBudget.resolved()
		top := ladderTop(definition.ModelBudget)
		if top != budget.MaxCompletionTokens {
			t.Errorf("%s declares max_completion_tokens %d and its ladder stops at %d "+
				"after %d raises from %d. Add a raise or lower the ceiling, because the "+
				"deployment reasons about the number it declares",
				filepath.Base(path), budget.MaxCompletionTokens, top,
				budget.BudgetRaises, budget.BaseCompletionTokens)
		}
	}
}
