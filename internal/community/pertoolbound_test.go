package community

import (
	"strings"
	"testing"
)

// A tool that names no bound keeps the budget-wide ceiling, so adding the
// mechanism changes no deployment until someone configures one. See #635.
func TestAnUnnamedToolInheritsTheBudgetCeiling(t *testing.T) {
	t.Parallel()
	budget := ModelBudget{ToolResultBytes: 8192}.resolved()
	if got := budget.ToolResultBytesFor("eco__get_market"); got != 8192 {
		t.Errorf("unnamed tool resolved to %d, want the budget ceiling 8192", got)
	}
}

// The whole point: two tools on one definition can differ.
func TestANamedToolOverridesTheBudgetCeiling(t *testing.T) {
	t.Parallel()
	budget := ModelBudget{
		ToolResultBytes:       8192,
		ToolResultBytesByTool: map[string]int{"steam__get_owned_games": 65536},
	}.resolved()
	if got := budget.ToolResultBytesFor("steam__get_owned_games"); got != 65536 {
		t.Errorf("named tool resolved to %d, want its own bound 65536", got)
	}
	if got := budget.ToolResultBytesFor("eco__get_market"); got != 8192 {
		t.Errorf("a sibling tool resolved to %d, want the ceiling 8192", got)
	}
}

// An override below the ceiling is as legitimate as one above it. A tool that
// returns one status line does not need the same room as a library dump.
func TestAnOverrideMayLowerTheBound(t *testing.T) {
	t.Parallel()
	budget := ModelBudget{
		ToolResultBytes:       16384,
		ToolResultBytesByTool: map[string]int{"eco__get_server_status": 512},
	}.resolved()
	if got := budget.ToolResultBytesFor("eco__get_server_status"); got != 512 {
		t.Errorf("resolved to %d, want the lower bound 512", got)
	}
}

// Zero is not unset here. An absent key inherits; a present one is deliberate.
func TestANonPositiveOverrideIsRefused(t *testing.T) {
	t.Parallel()
	for _, bound := range []int{0, -1} {
		budget := ModelBudget{
			ToolResultBytesByTool: map[string]int{"steam__get_owned_games": bound},
		}
		err := budget.validate()
		if err == nil {
			t.Errorf("a bound of %d was accepted, so a tool would deliver nothing", bound)
			continue
		}
		if !strings.Contains(err.Error(), "steam__get_owned_games") {
			t.Errorf("the error does not name the tool: %v", err)
		}
	}
}

// A definition naming no overrides at all is the deployed shape today.
func TestNoOverridesValidatesAndInherits(t *testing.T) {
	t.Parallel()
	budget := ModelBudget{ToolResultBytes: 16384}
	if err := budget.validate(); err != nil {
		t.Fatalf("a budget with no overrides was refused: %v", err)
	}
	if got := budget.resolved().ToolResultBytesFor("anything"); got != 16384 {
		t.Errorf("resolved to %d, want 16384", got)
	}
}
