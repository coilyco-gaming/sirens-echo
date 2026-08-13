package community

import (
	"strings"
	"testing"
)

// A gate that says a reply did not match, and will not show the reply, makes
// every failure a re-run rather than a diagnosis. See sirens-echo#386.

func TestAFailedCaseShowsTheParsedReply(t *testing.T) {
	t.Parallel()
	got := failedReply("Plain text, decision first.", CompletionResult{Content: "raw"})
	if got != "Plain text, decision first." {
		t.Errorf("failedReply = %q, want the parsed reply", got)
	}
}

// Scoring can fail before it has a parsed reply, and the raw completion is
// then the only evidence there is.
func TestTheRawCompletionStandsInWhenParsingFailed(t *testing.T) {
	t.Parallel()
	got := failedReply("", CompletionResult{Content: "  <tool_call>lookup</tool_call>  "})
	if got != "<tool_call>lookup</tool_call>" {
		t.Errorf("failedReply = %q, want the raw completion", got)
	}
}

// A model that returned nothing must say so, because an empty line under a
// fail heading reads as a formatting bug rather than as the finding.
func TestAnEmptyReplyIsNamedRatherThanBlank(t *testing.T) {
	t.Parallel()
	got := failedReply("   ", CompletionResult{Content: ""})
	if strings.TrimSpace(got) == "" {
		t.Fatal("a failing case printed nothing at all")
	}
	if !strings.Contains(got, "no content") {
		t.Errorf("failedReply = %q, want it to name the emptiness", got)
	}
}
