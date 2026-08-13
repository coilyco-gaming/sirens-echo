package community

import (
	"strings"
	"testing"
)

// URLs and Discord markup are excluded from mention resolution. A backtick span
// was neither, and the disclosure footer is made of them. See sirens-echo#486.

// footerWithToolNames is the reply shape that reaches sendReply on any turn
// that called a tool, which is where mention resolution runs.
func footerWithToolNames() string {
	return AppendToolDisclosure("Trading is busy right now.",
		ExecutedTool{Name: "eco.get_market", Outcome: ToolOutcomeOK},
		ExecutedTool{Name: "eco.find_trade", Outcome: ToolOutcomeEmpty})
}

// Was characterization, now regression. A tool name in the receipt is a tool,
// not a member who happens to share its first label.
func TestAToolNamedMemberLeavesTheFooterAlone(t *testing.T) {
	t.Parallel()
	roster := mentionRoster{}
	roster.add("eco", "999")
	content := footerWithToolNames()
	out, resolved := roster.resolveMentions(content)
	if out != content {
		t.Errorf("the receipt was rewritten:\n  in =%q\n  out=%q", content, out)
	}
	// Both tool names survive. One rewritten and one not was the defect, and it
	// left the receipt naming a person and a tool for the same call.
	if strings.Count(out, "eco.") != 2 {
		t.Errorf("expected both tool names intact, got:\n%s", out)
	}
	if len(resolved) != 0 {
		t.Errorf("resolved %v, so a receipt pinged someone", resolved)
	}
}

// The same collision in an ordinary code span, which is the general form.
func TestANameInsideACodeSpanIsLeftAlone(t *testing.T) {
	t.Parallel()
	roster := mentionRoster{}
	roster.add("eco", "999")
	for _, reply := range []string{
		"run `eco status` to check",
		"```\neco --version\n```",
	} {
		out, resolved := roster.resolveMentions(reply)
		if out != reply {
			t.Errorf("a code span was rewritten:\n  in =%q\n  out=%q", reply, out)
		}
		if len(resolved) != 0 {
			t.Errorf("%q pinged %v for a name inside code", reply, resolved)
		}
	}
}

// The half that must survive the fix. A name in prose is still a person, and
// emphasis or quotation around it does not change that.
func TestANameInProseStillResolvesBesideCode(t *testing.T) {
	t.Parallel()
	for _, reply := range []string{
		"eco confirmed it",
		"**eco** confirmed it",
		`she said "eco" earlier`,
	} {
		roster := mentionRoster{}
		roster.add("eco", "999")
		out, resolved := roster.resolveMentions(reply)
		if len(resolved) != 1 || !strings.Contains(out, "<@999>") {
			t.Errorf("a name in prose stopped resolving: %q -> %q %v", reply, out, resolved)
		}
	}
}
