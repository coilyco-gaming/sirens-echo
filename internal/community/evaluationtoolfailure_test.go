package community

import (
	"strings"
	"testing"
)

// A tool the model declined and a tool it never had fail identically from the
// result alone, and they are fixed differently. See sirens-echo#357.

func TestADeclinedToolStillReadsAsTheAgentsFailure(t *testing.T) {
	t.Parallel()
	err := missingToolFailure(CompletionResult{
		OfferedTools: []string{"eco__get_eco_server_status", "forgejo__create_issue"},
	}, "eco__get_eco_server_status")
	if err == nil {
		t.Fatal("a tool that was offered and not called is still a failure")
	}
	if got := err.Error(); got != "expected tool eco__get_eco_server_status" {
		t.Errorf("the offered case changed its wording: %q", got)
	}
}

// The sentence a reader acts on. "expected tool" sent someone looking for a
// model that ignored a tool, when the run had never offered one.
func TestAToolThatWasNeverOfferedSaysSo(t *testing.T) {
	t.Parallel()
	for name, result := range map[string]CompletionResult{
		"no roster at all":    {},
		"a roster without it": {OfferedTools: []string{"eco__get_eco_server_status"}},
	} {
		err := missingToolFailure(result, "forgejo__create_issue")
		if err == nil {
			t.Fatalf("%s produced no failure", name)
		}
		got := err.Error()
		if !strings.Contains(got, "never offered") {
			t.Errorf("%s does not say the tool was absent: %q", name, got)
		}
		// The distinction is the whole point, so the old wording must be gone.
		if strings.HasPrefix(got, "expected tool") {
			t.Errorf("%s still reads as the agent's failure: %q", name, got)
		}
		if !strings.Contains(got, "describes the run rather than the agent") {
			t.Errorf("%s does not say whose failure it is: %q", name, got)
		}
	}
}

// The count separates an empty roster from a populated one that lacks the
// tool, which read identically before and want different fixes.
func TestTheFailureCountsWhatWasOffered(t *testing.T) {
	t.Parallel()
	empty := missingToolFailure(CompletionResult{}, "forgejo__create_issue").Error()
	if !strings.Contains(empty, "(0 tools offered)") {
		t.Errorf("an empty roster is not reported as empty: %q", empty)
	}
	populated := missingToolFailure(CompletionResult{
		OfferedTools: []string{"a", "b", "c"},
	}, "forgejo__create_issue").Error()
	if !strings.Contains(populated, "(3 tools offered)") {
		t.Errorf("a populated roster is not counted: %q", populated)
	}
}

// wasOffered is what separates the two, so it is pinned directly.
func TestWasOfferedReadsTheOfferedList(t *testing.T) {
	t.Parallel()
	result := CompletionResult{OfferedTools: []string{"eco__get_trades"}}
	if !result.wasOffered("eco__get_trades") {
		t.Error("an offered tool reported as absent")
	}
	if result.wasOffered("eco__get_market") {
		t.Error("an absent tool reported as offered")
	}
	if (CompletionResult{}).wasOffered("anything") {
		t.Error("a turn with no tools reported one as offered")
	}
}
