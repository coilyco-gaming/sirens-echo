package community

import (
	"fmt"
	"strings"
	"testing"
)

// A budget failure has to say which failure it was: a model that thought and
// ran out, or one that produced nothing. See sirens-echo#325.

func TestTruncatedNeedsEmptyContentAndNoToolCalls(t *testing.T) {
	t.Parallel()
	spent := chatChoice{FinishReason: finishReasonLength}
	spent.Message.ReasoningContent = "a long deliberation"
	if !spent.truncated() {
		t.Error("a length finish with no content is not being treated as truncation")
	}
	// A length finish that still produced an answer is usable, and treating it
	// as truncation would discard a reply a member could have read.
	usable := chatChoice{FinishReason: finishReasonLength}
	usable.Message.Content.Text = "The Eco server is online."
	if usable.truncated() {
		t.Error("a length finish carrying content was treated as truncation")
	}
	withTool := chatChoice{FinishReason: finishReasonLength}
	withTool.Message.ToolCalls = []chatToolCall{{}}
	if withTool.truncated() {
		t.Error("a length finish carrying a tool call was treated as truncation")
	}
}

// The two failures are indistinguishable without this number, and they want
// opposite responses: raise the ceiling, or look at the model.
func TestBudgetFailureDistinguishesThoughtFromSilence(t *testing.T) {
	t.Parallel()
	thought := chatChoice{FinishReason: finishReasonLength}
	thought.Message.ReasoningContent = strings.Repeat("deliberating. ", 40)
	silent := chatChoice{FinishReason: finishReasonLength}

	thoughtBytes := len(strings.TrimSpace(thought.Message.ReasoningContent))
	silentBytes := len(strings.TrimSpace(silent.Message.ReasoningContent))

	if thoughtBytes == 0 {
		t.Fatal("a model that reasoned reports no reasoning bytes")
	}
	if silentBytes != 0 {
		t.Fatal("a model that produced nothing reports reasoning bytes")
	}
	if thoughtBytes == silentBytes {
		t.Fatal("the two failures are still indistinguishable")
	}
}

// The count is a length, never the text. Reasoning content is model output and
// the turn logger carries no model bodies.
func TestBudgetFailureCarriesNoReasoningText(t *testing.T) {
	t.Parallel()
	secret := "the operator handle is coilysiren and the plan is"
	choice := chatChoice{FinishReason: finishReasonLength}
	choice.Message.ReasoningContent = secret
	size := len(strings.TrimSpace(choice.Message.ReasoningContent))
	rendered := formatBudgetExhausted(3600, 2, size).Error()
	if strings.Contains(rendered, "coilysiren") || strings.Contains(rendered, "plan") {
		t.Errorf("the failure leaks reasoning text: %q", rendered)
	}
	if !strings.Contains(rendered, fmt.Sprintf("%d bytes of reasoning", size)) {
		t.Errorf("the failure does not report the reasoning size: %q", rendered)
	}
}
