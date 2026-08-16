package community

import (
	"strings"
	"testing"
)

// The reply path refuses unparsed tool-call markup rather than stripping it.
// See docs/sirens-echo-tool-call-markup.md.

// The observed reply, verbatim from evaluations/eval-deep-run1.yaml. A member
// would have read this.
const observedMarkupReply = "I'll check the issue tracker in the repo for recent announcements.\n\n" +
	"<｜｜DSML｜｜tool_calls>\n" +
	"<｜｜DSML｜｜invoke name=\"list_issue\">\n" +
	"<｜｜DSML｜｜parameter name=\"state\" string=\"true\">open</｜｜DSML｜｜parameter>\n" +
	"</｜｜DSML｜｜invoke>\n" +
	"</｜｜DSML｜｜tool_calls>"

func TestReplyPathRefusesToolCallMarkup(t *testing.T) {
	t.Parallel()
	for name, reply := range map[string]string{
		"observed deepseek": observedMarkupReply,
		"hermes style":      "Checking now.\n<tool_call>{\"name\": \"list_issue\"}</tool_call>",
		"anthropic style":   "One moment.\n<invoke name=\"list_issue\">",
		"control token":     "Working on it. <|python_tag|>",
	} {
		name, reply := name, reply
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			if err := ValidateNoToolCallMarkup(reply); err == nil {
				t.Fatal("a member would have read raw tool-call markup")
			}
		})
	}
}

// The error must not carry the markup itself. Reply bodies never enter the turn
// logger, and a rejection is logged.
func TestMarkupRefusalCarriesNoModelOutput(t *testing.T) {
	t.Parallel()
	err := ValidateNoToolCallMarkup(observedMarkupReply)
	if err == nil {
		t.Fatal("expected a refusal")
	}
	for _, leaked := range []string{"DSML", "list_issue", "issue tracker", "invoke"} {
		if strings.Contains(err.Error(), leaked) {
			t.Errorf("the error leaks model output: %q contains %q", err.Error(), leaked)
		}
	}
}

// The must-not-fire half. Discussing tool calls is correct and common, including
// in this repository's own debugging threads.
func TestReplyPathAcceptsRepliesThatOnlyDiscussToolCalls(t *testing.T) {
	t.Parallel()
	for _, reply := range []string{
		"The harness emits tool_calls as a structured field rather than as content.",
		"Set `max_context_messages` to 12 and the invoke count drops.",
		"A tool call failed, so the answer is unverified.",
		"Run `just test` to check the invoke path.",
		"The Eco server is online.",
	} {
		reply := reply
		t.Run(reply[:20], func(t *testing.T) {
			t.Parallel()
			if err := ValidateNoToolCallMarkup(reply); err != nil {
				t.Fatalf("refused a correct reply: %q (%v)", reply, err)
			}
		})
	}
}

// One definition, two readers. The gate and the reply path must agree on what
// markup is, or one of them stops seeing what the other rejects.
func TestReplyPathAndGateShareOneMarkupDefinition(t *testing.T) {
	t.Parallel()
	if err := checkToolCallMarkup(observedMarkupReply); err == nil {
		t.Fatal("the gate check no longer sees the reply the reply path refuses")
	}
	if !containsToolCallMarkup(observedMarkupReply) {
		t.Fatal("the reply-path predicate disagrees with the gate check")
	}
	clean := "The harness emits tool_calls as a structured field."
	if checkToolCallMarkup(clean) == nil != !containsToolCallMarkup(clean) {
		t.Error("the two readers disagree on a clean reply")
	}
}

// ParseReply must stay clear of this. The evaluation scorer and the repair loop
// both call it, and neither may have its measurement reshaped by a runtime gate.
func TestParseReplyStillAcceptsToolCallMarkup(t *testing.T) {
	t.Parallel()
	if _, err := ParseReply(observedMarkupReply); err != nil {
		t.Fatalf("ParseReply gained a markup check, which changes what the "+
			"gate and the rate pack measure: %v", err)
	}
}
