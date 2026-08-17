package community

import (
	"path/filepath"
	"testing"
)

// A rate case blind to tool-call markup reports a rate over replies that were
// never answers. See docs/sirens-echo-tool-markup.md and sirens-echo#301.

// The gate keeps the flag opt-in on purpose. A rate gates nothing, so that
// reason does not reach it. See docs/sirens-echo-tool-markup.md.
func TestEveryRateCaseChecksToolCallMarkup(t *testing.T) {
	t.Parallel()
	packs := globAll(t,
		[]string{"..", "..", "agent", "rate-*.yaml"},
		[]string{"..", "..", "agents", "*", "packs", "rate*.yaml"},
	)
	for _, path := range packs {
		pack, err := LoadRatePack(path)
		if err != nil {
			t.Fatalf("load %s: %v", path, err)
		}
		for _, rateCase := range pack.Cases {
			if rateCase.ForbidToolCallMarkup {
				continue
			}
			t.Errorf("rate case %s in %s does not check tool-call markup. A run "+
				"where the model emits its own delimiters would score as a pass, so "+
				"the rate would describe replies that were not answers. Set "+
				"forbid_tool_call_markup: true",
				rateCase.ID, filepath.Base(path))
		}
	}
}

// The failure this guards is not hypothetical: it happened to the case that
// found it, which reported 10 of 10 while nine replies were markup.
func TestAMarkupReplyFailsARateCaseThatDeclaresNothingElseAboutIt(t *testing.T) {
	t.Parallel()
	subject := EvaluationCase{
		ID:                   "probe",
		Current:              TranscriptEntry{Author: "member", Content: "is there a ticket open?"},
		ForbidToolCallMarkup: true,
		ForbiddenPatterns:    []string{`(?i)https?://[^\s]+/issues/\d+`},
	}
	if err := prepareEvaluationCase(&subject); err != nil {
		t.Fatalf("prepare: %v", err)
	}
	markup := "Checking now.\n<｜｜DSML｜｜tool_calls>\n<｜｜DSML｜｜invoke name=\"search_issues\">"
	if _, err := ScoreEvaluationCase(
		subject,
		CompletionResult{Content: markup},
		TurnPrompt{System: "policy", Message: "is there a ticket open?"},
		"policy", ResponseStyleSocial, "", nil, Principal{},
	); err == nil {
		t.Error("a reply that is entirely tool-call markup scored as a pass, which " +
			"is the vacuous rate this guard exists to prevent")
	}
	// The control, since a check that fails everything measures nothing either.
	if _, err := ScoreEvaluationCase(
		subject,
		CompletionResult{Content: "Nothing in this turn returned a tracker result."},
		TurnPrompt{System: "policy", Message: "is there a ticket open?"},
		"policy", ResponseStyleSocial, "", nil, Principal{},
	); err != nil {
		t.Errorf("an ordinary reply was scored a failure: %v", err)
	}
}
