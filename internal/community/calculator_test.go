package community

import (
	"context"
	"strings"
	"testing"
)

// Every number Echo computed herself was predicted rather than calculated. See
// sirens-echo#916.

func calculate(t *testing.T, expression string) ToolResult {
	t.Helper()
	session, err := (&CalculatorProvider{}).Open(context.Background())
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	result, err := session.Call(
		context.Background(), calculatorToolName,
		map[string]any{"expression": expression},
	)
	if err != nil {
		t.Fatalf("Call(%q): %v", expression, err)
	}
	return result
}

// The arithmetic a member's question actually asks for.
func TestTheCalculatorAnswersArithmetic(t *testing.T) {
	t.Parallel()
	for _, testCase := range []struct{ expression, want string }{
		{"2 + 2", "4"},
		{"(1250 + 90) / 4", "335"},
		{"15% * 240", "36"},
		{"1,250 * 3", "3750"},
		{"2^10", "1024"},
		{"-3 * -4", "12"},
		{"10 / 4", "2.5"},
		{"3 - 5", "-2"},
		// Right associative, so this is 2^9 rather than 64^2.
		{"2^3^2", "512"},
	} {
		testCase := testCase
		t.Run(testCase.expression, func(t *testing.T) {
			t.Parallel()
			result := calculate(t, testCase.expression)
			if result.IsError {
				t.Fatalf("refused: %s", result.Text)
			}
			if got := strings.TrimPrefix(result.Text, testCase.expression+" = "); got != testCase.want {
				t.Errorf("%s = %q, want %q", testCase.expression, got, testCase.want)
			}
		})
	}
}

// Exact rather than floating point, which is the difference between a
// calculator and a faster way to be wrong.
func TestTheCalculatorIsExact(t *testing.T) {
	t.Parallel()
	result := calculate(t, "0.1 + 0.2")
	if result.IsError {
		t.Fatalf("refused: %s", result.Text)
	}
	if !strings.HasSuffix(result.Text, "= 0.3") {
		t.Errorf("0.1 + 0.2 = %q, want exactly 0.3", result.Text)
	}
}

// A number that is not a decimal says it was rounded, because a rounded value
// presented as exact is the failure this tool exists to stop.
func TestAnInexactResultSaysSo(t *testing.T) {
	t.Parallel()
	result := calculate(t, "1 / 3")
	if result.IsError {
		t.Fatalf("refused: %s", result.Text)
	}
	if !strings.Contains(result.Text, "rounded") {
		t.Errorf("a third was reported without saying it was rounded: %q", result.Text)
	}
	if !strings.Contains(result.Text, "1/3") {
		t.Errorf("the exact value is not reported: %q", result.Text)
	}
}

// The expression is echoed, so a trace can be checked against what was asked.
func TestTheCalculatorEchoesTheExpression(t *testing.T) {
	t.Parallel()
	result := calculate(t, "7 * 6")
	if !strings.HasPrefix(result.Text, "7 * 6 = ") {
		t.Errorf("the expression is not echoed: %q", result.Text)
	}
}

// Bad arguments refuse as a readable result rather than failing the turn, the
// way fetch_url and the scratch tools already do.
func TestTheCalculatorRefusesRatherThanFailingTheTurn(t *testing.T) {
	t.Parallel()
	for _, testCase := range []struct{ name, expression, reason string }{
		{"empty", "   ", "required"},
		{"divide by zero", "5 / 0", "zero"},
		{"unclosed bracket", "(2 + 3", "bracket"},
		{"not arithmetic", "rm -rf /", "not arithmetic"},
		{"an identifier", "total * 2", "not arithmetic"},
		{"a function call", "exec(1)", "not arithmetic"},
		{"trailing operator", "2 +", "stops before"},
		{"fractional exponent", "9^0.5", "root"},
		{"too long", strings.Repeat("1+", maxCalculatorRunes) + "1", "longer than"},
		{"an enormous power", "2^100000", "exponent beyond"},
	} {
		testCase := testCase
		t.Run(testCase.name, func(t *testing.T) {
			t.Parallel()
			result := calculate(t, testCase.expression)
			if !result.IsError {
				t.Fatalf("%q was accepted as %q", testCase.expression, result.Text)
			}
			if !strings.Contains(result.Text, testCase.reason) {
				t.Errorf("refusal %q does not say %q", result.Text, testCase.reason)
			}
		})
	}
}

// A name the tool does not serve is a harness fault rather than a member one,
// so it fails the turn instead of coming back as a readable refusal.
func TestAnUnknownCalculatorToolIsAnError(t *testing.T) {
	t.Parallel()
	session, err := (&CalculatorProvider{}).Open(context.Background())
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	if _, err := session.Call(context.Background(), "evaluate", nil); err == nil {
		t.Error("an unserved tool name was accepted")
	}
}

// The tool has to reach the model, and it carries no grounding or guidance of
// its own the way a roster server does.
func TestTheCalculatorOffersOneTool(t *testing.T) {
	t.Parallel()
	session, err := (&CalculatorProvider{}).Open(context.Background())
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	tools := session.Tools()
	if len(tools) != 1 || tools[0].Name != calculatorToolName {
		t.Fatalf("tools = %#v", tools)
	}
	if tools[0].Server != calculatorToolServer {
		t.Errorf("server = %q", tools[0].Server)
	}
	if len(session.Grounding()) != 0 || len(session.Guidance()) != 0 {
		t.Error("the calculator volunteers reference material")
	}
}
