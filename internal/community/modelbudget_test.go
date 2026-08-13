package community

import (
	"path/filepath"
	"reflect"
	"strings"
	"testing"
)

// The two profiles do not share a substrate, so they cannot share one ceiling.
// See sirens-echo#467.

// The property the whole change rests on. A definition that names no budget
// behaves exactly as the constants did, so nothing moves by accident.
func TestAnUnsetBudgetIsThePackagedDefault(t *testing.T) {
	t.Parallel()
	resolved := ModelBudget{}.resolved()
	for _, testCase := range []struct {
		name string
		got  int
		want int
	}{
		{"tool_rounds", resolved.ToolRounds, maxToolRounds},
		{"base_completion_tokens", resolved.BaseCompletionTokens, baseCompletionTokens},
		{"max_completion_tokens", resolved.MaxCompletionTokens, maxCompletionTokens},
		{"budget_raises", resolved.BudgetRaises, budgetRaisesAllowed},
		{"tool_result_bytes", resolved.ToolResultBytes, maxToolResultBytes},
	} {
		if testCase.got != testCase.want {
			t.Errorf("%s = %d, want the packaged %d",
				testCase.name, testCase.got, testCase.want)
		}
	}
}

// A definition names only what it changes, so one raised field does not drag
// the others off their defaults.
func TestOneRaisedFieldLeavesTheRestPackaged(t *testing.T) {
	t.Parallel()
	resolved := ModelBudget{MaxCompletionTokens: 14400}.resolved()
	if resolved.MaxCompletionTokens != 14400 {
		t.Errorf("max_completion_tokens = %d, want 14400", resolved.MaxCompletionTokens)
	}
	if resolved.ToolRounds != maxToolRounds {
		t.Errorf("tool_rounds = %d, want the packaged %d",
			resolved.ToolRounds, maxToolRounds)
	}
}

// A ProxyClient built without a budget is what every test and the evaluation
// runner construct, so the zero value has to be the default rather than zero.
func TestAProxyClientWithNoBudgetUsesTheDefaults(t *testing.T) {
	t.Parallel()
	if got := (ProxyClient{}).budget().ToolRounds; got != maxToolRounds {
		t.Errorf("tool rounds = %d, want the packaged %d", got, maxToolRounds)
	}
	if got := (ProxyClient{}).budget().MaxCompletionTokens; got != maxCompletionTokens {
		t.Errorf("ceiling = %d, want the packaged %d", got, maxCompletionTokens)
	}
}

// Every field is a ceiling, so a negative one would spend without bound or
// refuse everything. Neither is a thing a definition should be able to say.
func TestANegativeBudgetFieldIsRefused(t *testing.T) {
	t.Parallel()
	for _, budget := range []ModelBudget{
		{ToolRounds: -1},
		{BaseCompletionTokens: -1},
		{MaxCompletionTokens: -1},
		{BudgetRaises: -1},
		{ToolResultBytes: -1},
	} {
		if err := budget.validate(); err == nil {
			t.Errorf("%+v was accepted", budget)
		}
	}
}

// A ceiling below the floor is a ladder that cannot climb, which reads as a
// budget and behaves as exhaustion on the first call.
func TestACeilingBelowTheFloorIsRefused(t *testing.T) {
	t.Parallel()
	err := ModelBudget{BaseCompletionTokens: 4000, MaxCompletionTokens: 2000}.validate()
	if err == nil {
		t.Fatal("a ceiling below the floor was accepted")
	}
	if !strings.Contains(err.Error(), "below base_completion_tokens") {
		t.Errorf("error does not name the contradiction: %v", err)
	}
}

// The raised ladder has to actually climb to the ceiling it names, or the
// ceiling is decoration. Deep's is 3600 to 7200 to 14400.
func TestARaisedLadderReachesItsCeiling(t *testing.T) {
	t.Parallel()
	budget := ModelBudget{
		BaseCompletionTokens: 3600,
		MaxCompletionTokens:  14400,
		BudgetRaises:         2,
	}.resolved()

	current := budget.BaseCompletionTokens
	for raise := 0; raise < budget.BudgetRaises; raise++ {
		next, ok := nextCompletionBudget(current, budget.MaxCompletionTokens)
		if !ok {
			t.Fatalf("raise %d could not climb from %d", raise+1, current)
		}
		current = next
	}
	if current != budget.MaxCompletionTokens {
		t.Errorf("the ladder stopped at %d, short of its %d ceiling",
			current, budget.MaxCompletionTokens)
	}
	if _, ok := nextCompletionBudget(current, budget.MaxCompletionTokens); ok {
		t.Error("the ladder climbed past its ceiling")
	}
}

// The shipped definitions, because the point of the change is that these two
// differ. Echo runs a 35B model on the daily driver and must not be raised.
func TestTheShippedDefinitionsDoNotShareACeiling(t *testing.T) {
	t.Parallel()
	echo, err := LoadDefinition(filepath.Join("..", "..", "agent", "sirens-echo.yaml"))
	if err != nil {
		t.Fatalf("load the Echo definition: %v", err)
	}
	deep, err := LoadDefinition(filepath.Join("..", "..", "agent", "sirens-deep.yaml"))
	if err != nil {
		t.Fatalf("load the Deep definition: %v", err)
	}

	// reflect.DeepEqual since #635 gave the budget a map field. The assertion
	// is unchanged: Echo names nothing and inherits every packaged default.
	if !reflect.DeepEqual(echo.ModelBudget, ModelBudget{}) {
		t.Errorf("Echo names a budget %+v, and its route is the tower",
			echo.ModelBudget)
	}
	if echo.ModelBudget.resolved().MaxCompletionTokens != maxCompletionTokens {
		t.Error("Echo's ceiling moved off the packaged default")
	}
	if deep.ModelBudget.resolved().MaxCompletionTokens <= maxCompletionTokens {
		t.Errorf("Deep's ceiling is %d, which is not a raise",
			deep.ModelBudget.resolved().MaxCompletionTokens)
	}
	// The ask was 2x to 4x. A raise outside that is a different decision.
	ratio := float64(deep.ModelBudget.resolved().MaxCompletionTokens) /
		float64(maxCompletionTokens)
	if ratio < 2 || ratio > 4 {
		t.Errorf("Deep's ceiling is %.1fx the default, outside the 2x to 4x asked for",
			ratio)
	}
}

// A definition carrying a malformed budget is refused at load rather than
// producing a turn that spends what it should not.
func TestAMalformedBudgetIsRefusedAtLoad(t *testing.T) {
	t.Parallel()
	body := validDefinition + `model_budget:
  tool_rounds: -3
`
	if _, err := LoadDefinition(writeDefinition(t, body)); err == nil {
		t.Fatal("a definition with a negative ceiling loaded")
	}
}

// A ceiling the rungs stop short of is a number that never applies, which is
// the argument the raise itself was made on. See sirens-echo#522.
func TestACeilingTheLadderCannotReachIsRefused(t *testing.T) {
	t.Parallel()
	for _, testCase := range []struct {
		name    string
		budget  ModelBudget
		refused bool
	}{
		{
			"the reported case, a raised ceiling without a rung",
			ModelBudget{BaseCompletionTokens: 3600, MaxCompletionTokens: 14400, BudgetRaises: 1},
			true,
		},
		{
			"Deep's shipped budget",
			ModelBudget{BaseCompletionTokens: 3600, MaxCompletionTokens: 14400, BudgetRaises: 2},
			false,
		},
		{
			"the packaged defaults",
			ModelBudget{},
			false,
		},
		{
			"slack upward, where the ceiling binds below the rung",
			ModelBudget{BaseCompletionTokens: 1800, MaxCompletionTokens: 5000, BudgetRaises: 2},
			false,
		},
		{
			"a ladder with nowhere to climb, which is how never-raise is written",
			ModelBudget{BaseCompletionTokens: 3600, MaxCompletionTokens: 3600, BudgetRaises: 1},
			false,
		},
		{
			"one token above the rung",
			ModelBudget{BaseCompletionTokens: 1800, MaxCompletionTokens: 3601, BudgetRaises: 1},
			true,
		},
	} {
		err := testCase.budget.validate()
		if testCase.refused && err == nil {
			t.Errorf("%s: accepted a ceiling the ladder stops short of", testCase.name)
		}
		if !testCase.refused && err != nil {
			t.Errorf("%s: refused, %v", testCase.name, err)
		}
	}
}

// The refusal has to name the rung, or it tells a deployment its number is
// wrong without saying what would be right.
func TestTheUnreachableCeilingErrorNamesWhereTheLadderStops(t *testing.T) {
	t.Parallel()
	err := ModelBudget{
		BaseCompletionTokens: 3600, MaxCompletionTokens: 14400, BudgetRaises: 1,
	}.validate()
	if err == nil {
		t.Fatal("accepted")
	}
	for _, want := range []string{"14400", "7200", "unreachable"} {
		if !strings.Contains(err.Error(), want) {
			t.Errorf("error does not carry %q: %v", want, err)
		}
	}
}

// A large raise count must not overflow on the way to a small ceiling.
func TestTheLadderWalkStopsOnceItHasReachedTheCeiling(t *testing.T) {
	t.Parallel()
	budget := ModelBudget{
		BaseCompletionTokens: 1800, MaxCompletionTokens: 3600, BudgetRaises: 1000,
	}.resolved()
	if got := budget.ladderTop(); got != 3600 {
		t.Errorf("ladderTop = %d, want it to stop at the ceiling", got)
	}
	if err := budget.validate(); err != nil {
		t.Errorf("refused a reachable ceiling: %v", err)
	}
}
