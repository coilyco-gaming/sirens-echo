package community

import "testing"

// A base below the model's reasoning floor makes the first call structurally
// unable to succeed, so the turn pays for two. See sirens-echo#360.

// The measured floor: raises reported 4138, 4219, and 4374 bytes of reasoning
// before any content, which is on the order of a thousand tokens.
const observedReasoningBytes = 4374

func TestTheBaseBudgetClearsTheObservedReasoningFloor(t *testing.T) {
	t.Parallel()
	// Four bytes per token is the conservative end of the usual ratio, so this
	// under-counts the reasoning rather than flattering the budget.
	floor := observedReasoningBytes / 4
	if baseCompletionTokens <= floor {
		t.Errorf(
			"base %d is at or below the observed reasoning floor of about %d tokens",
			baseCompletionTokens, floor,
		)
	}
}

// A raise that cannot raise would retry at the same budget, which repeats the
// wall the first call hit. Raising the base is what made this reachable.
func TestARaiseThatCannotRaiseIsExhaustion(t *testing.T) {
	t.Parallel()
	if raised, ok := nextCompletionBudget(maxCompletionTokens); ok {
		t.Errorf("a budget already at the ceiling raised to %d", raised)
	}
	// Above the ceiling is the same answer. Nothing sets this today, and a
	// future clamp change should not turn it into an identical retry.
	if _, ok := nextCompletionBudget(maxCompletionTokens + 1); ok {
		t.Error("a budget above the ceiling reported a real raise")
	}
}

func TestTheLadderStillClimbsFromTheBase(t *testing.T) {
	t.Parallel()
	raised, ok := nextCompletionBudget(baseCompletionTokens)
	if !ok {
		t.Fatal("the base cannot raise at all, so the safety net is gone")
	}
	if raised <= baseCompletionTokens {
		t.Errorf("raise produced %d from a base of %d", raised, baseCompletionTokens)
	}
	if raised > maxCompletionTokens {
		t.Errorf("raise produced %d, above the ceiling of %d", raised, maxCompletionTokens)
	}
}

// Walking the ladder to its end proves it terminates and never repeats a
// budget, whatever the base and ceiling are later set to.
func TestTheLadderTerminatesAndNeverRepeatsABudget(t *testing.T) {
	t.Parallel()
	seen := map[int]bool{baseCompletionTokens: true}
	budget := baseCompletionTokens
	for step := 0; step < budgetRaisesAllowed; step++ {
		raised, ok := nextCompletionBudget(budget)
		if !ok {
			break
		}
		if seen[raised] {
			t.Fatalf("the ladder returned to %d, so a retry repeats the wall", raised)
		}
		seen[raised] = true
		budget = raised
	}
	if _, ok := nextCompletionBudget(maxCompletionTokens); ok {
		t.Error("the ladder does not terminate at the ceiling")
	}
}
