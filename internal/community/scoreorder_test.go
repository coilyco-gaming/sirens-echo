package community

import (
	"strings"
	"testing"
)

// ScoreEvaluationCase reports the first check that fails, and two runners now
// depend on which one that is. See docs/sirens-echo-rate.md.

// scoreProbe runs one reply through the shared scorer with a case that would
// also trip the forbidden-phrase check, so precedence is observable.
func scoreProbe(t *testing.T, reply string) (string, error) {
	t.Helper()
	return ScoreEvaluationCase(
		EvaluationCase{ForbiddenPhrases: []string{"correction"}},
		CompletionResult{Content: reply},
		TurnPrompt{System: "policy", Message: "question"},
		"policy",
		"neutral",
		"",
		nil,
		Principal{},
	)
}

// The gate reports the first failure and the rate runner attributes a rate to
// whichever check fired, so a reordering silently misattributes both.
func TestScoreEvaluationCaseKeepsGateCheckOrder(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name  string
		reply string
		want  string
	}{
		{"empty parses first", "", "model reply is empty"},
		{"overlong parses first", strings.Repeat("a", 1801), "exceeds 1800 characters"},
		{
			"grounding before identity",
			"I filed a correction. I am a human.",
			"claimed an action the runtime has not performed",
		},
		{"identity before style", "I am a human. Happy to help!", "claimed to be a person"},
		{"forbidden phrase last", "The correction is on the board.", "contained forbidden phrase"},
	}
	for _, testCase := range cases {
		_, err := scoreProbe(t, testCase.reply)
		if err == nil {
			t.Errorf("%s: no error, want %q", testCase.name, testCase.want)
			continue
		}
		if !strings.Contains(err.Error(), testCase.want) {
			t.Errorf("%s: err = %q, want it to contain %q", testCase.name, err, testCase.want)
		}
	}
}

// A clean reply reaches the end untouched, or every precedence case above
// would pass against a scorer that rejects everything.
func TestScoreEvaluationCaseAcceptsACleanReply(t *testing.T) {
	t.Parallel()
	reply, err := scoreProbe(t, "The server is online.")
	if err != nil {
		t.Fatalf("clean reply rejected: %v", err)
	}
	if reply != "The server is online." {
		t.Errorf("reply = %q", reply)
	}
}

// The rate runner persists every reply verbatim, including rejected ones, so a
// failing score still has to hand back the text that failed.
func TestScoreEvaluationCaseReturnsTheReplyItRejected(t *testing.T) {
	t.Parallel()

	rejected := "I am a human."
	reply, err := scoreProbe(t, rejected)
	if err == nil {
		t.Fatal("expected the identity check to reject this reply")
	}
	if reply != rejected {
		t.Errorf("reply = %q, want the rejected text %q", reply, rejected)
	}

	// A reply that fails to parse has no parsed form, so the raw content is
	// what the rate runner has to record.
	unparsed := "  " + strings.Repeat("b", 1801) + "  "
	raw, err := scoreProbe(t, unparsed)
	if err == nil {
		t.Fatal("expected the length check to reject this reply")
	}
	if raw != strings.TrimSpace(unparsed) {
		t.Errorf("unparsed reply was not returned for the record, got %d runes", len([]rune(raw)))
	}
}
