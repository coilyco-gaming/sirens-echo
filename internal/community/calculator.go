package community

import (
	"context"
	"fmt"
	"math/big"
	"strings"
)

// Arithmetic Echo performs rather than predicts. See docs/sirens-echo-tools.md.

const (
	calculatorToolServer = "calculator"
	calculatorToolName   = "calculate"
)

// CalculatorProvider exposes exact arithmetic as a tool. In process, because it
// needs no network, no auth, and no egress bound.
type CalculatorProvider struct{}

// Open needs no per-turn state. The bound is the expression, not the requester.
func (p *CalculatorProvider) Open(context.Context) (ToolSession, error) {
	return &calculatorSession{}, nil
}

type calculatorSession struct{}

func (s *calculatorSession) Close() error { return nil }

// Grounding is empty: a calculation answers a question rather than volunteering
// reference material the way a server's resources do.
func (s *calculatorSession) Grounding() []GroundingDocument { return nil }

func (s *calculatorSession) Guidance() []ServerGuidance { return nil }

func (s *calculatorSession) Unavailable() []string { return nil }

func (s *calculatorSession) Tools() []ToolDefinition {
	return []ToolDefinition{{
		Name:     calculatorToolName,
		Original: calculatorToolName,
		Server:   calculatorToolServer,
		Description: "Evaluate arithmetic exactly and return the result. Use this for " +
			"every number in a reply that was not already in a tool result: totals, " +
			"rates, unit prices, scaling a recipe, and comparing two figures. " +
			"Supports + - * / ^, parentheses, and a trailing % meaning per cent. " +
			"It is not a programming language and evaluates nothing else.",
		InputSchema: scratchObjectSchema(map[string]any{
			"expression": scratchStringProperty(
				"Arithmetic to evaluate, for example (1250 + 90) / 4 or 15% * 240."),
		}, []string{"expression"}),
	}}
}

// Call evaluates one expression. A refusal comes back as an error result the
// model can read and correct, not as a Go error, which would fail the turn.
func (s *calculatorSession) Call(
	_ context.Context,
	name string,
	arguments map[string]any,
) (ToolResult, error) {
	if name != calculatorToolName {
		return ToolResult{}, fmt.Errorf("model requested unavailable calculator tool %q", name)
	}
	expression := strings.TrimSpace(scratchStringArg(arguments, "expression"))
	if expression == "" {
		return scratchRefusal("an expression is required")
	}
	if len([]rune(expression)) > maxCalculatorRunes {
		return scratchRefusal(
			"that expression is longer than %d characters", maxCalculatorRunes)
	}
	value, err := evaluateArithmetic(expression)
	if err != nil {
		return scratchRefusal("%s", err.Error())
	}
	// The expression is echoed, so a reader of the trace can check the answer
	// against what was asked rather than against what the reply claimed.
	return ToolResult{Text: expression + " = " + formatRational(value)}, nil
}

// formatRational prints an exact value exactly and says so when it cannot. A
// rounded number presented as exact is the failure this tool exists to stop.
func formatRational(value *big.Rat) string {
	if value.IsInt() {
		return value.Num().String()
	}
	rounded := strings.TrimRight(value.FloatString(maxCalculatorPlaces), "0")
	rounded = strings.TrimSuffix(rounded, ".")
	exact, ok := new(big.Rat).SetString(rounded)
	if ok && exact.Cmp(value) == 0 {
		return rounded
	}
	return fmt.Sprintf("%s (rounded to %d decimal places, exactly %s)",
		rounded, maxCalculatorPlaces, value.RatString())
}
