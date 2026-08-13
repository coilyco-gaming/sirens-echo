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
	resolved := budget.resolved()
	tokens := resolved.BaseCompletionTokens
	for raise := 0; raise < resolved.BudgetRaises; raise++ {
		next, canRaise := nextCompletionBudget(tokens, resolved.MaxCompletionTokens)
		if !canRaise {
			break
		}
		tokens = next
	}
	return tokens
}

// Every shipped definition, so raising a ceiling without adding a rung fails
// here rather than granting headroom the runtime never uses.
func TestEveryDefinitionsLadderReachesItsCeiling(t *testing.T) {
	t.Parallel()
	definitions, err := filepath.Glob(filepath.Join("..", "..", "agent", "sirens-*.yaml"))
	if err != nil || len(definitions) == 0 {
		t.Fatalf("glob definitions: %v, found %d", err, len(definitions))
	}
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

// The per-turn model-call ceiling is what deploy#431's rate limits are set
// against, so it is arithmetic another repository depends on.
func TestTheModelCallCeilingIsWhatTheDeploymentWasToldFor(t *testing.T) {
	t.Parallel()
	deep, err := LoadDefinition(filepath.Join("..", "..", "agent", "sirens-deep.yaml"))
	if err != nil {
		t.Fatalf("load the deep definition: %v", err)
	}
	budget := deep.ModelBudget.resolved()
	calls := budget.ToolRounds + maxResponseRepairs + budget.BudgetRaises + 1
	const told = 16
	if calls != told {
		t.Errorf("Deep's per-turn model-call ceiling is %d and deploy#431 was told %d. "+
			"Update that issue in the same change, because its rate limits are derived "+
			"from this number", calls, told)
	}
}
