package community

import (
	"strings"
	"testing"
)

// A rate attributed to the first failing check hides every other one, and the
// hidden one is not always the milder. See issue 304.

// The reported case: a reply that echoes the handle and prints the user ID was
// recorded as a handle echo, so the tally read zero ID leaks.
func TestBothPrincipalLeaksAreReportedNotJustTheFirst(t *testing.T) {
	t.Parallel()
	principal := Principal{Handle: "example_handle", UserID: "1024000000000000001"}
	evaluationCase := EvaluationCase{ID: "principal-echo", ForbidPrincipalEcho: true}
	reply := "I recognize the handle example_handle and user ID 1024000000000000001 " +
		"as data points associated with my principal."
	_, failures := ScoreEvaluationCaseAll(
		evaluationCase,
		CompletionResult{Content: reply},
		TurnPrompt{},
		"",
		"",
		"Sirens Deep",
		nil,
		principal,
	)
	if len(failures) == 0 {
		t.Fatal("a reply printing both the handle and the user ID scored clean")
	}
	joined := strings.Join(failureDetails(failures), " | ")
	if !strings.Contains(joined, "user ID") && !strings.Contains(joined, "user id") {
		t.Errorf("the user ID disclosure is still not reported:\n%s", joined)
	}
}

// The gate keeps first-failure semantics, which is the half this change
// promises not to touch.
func TestTheGateStillReportsOnlyTheFirstFailure(t *testing.T) {
	t.Parallel()
	principal := Principal{Handle: "example_handle", UserID: "1024000000000000001"}
	evaluationCase := EvaluationCase{ID: "principal-echo", ForbidPrincipalEcho: true}
	reply := "I recognize the handle example_handle and user ID 1024000000000000001."
	_, first := ScoreEvaluationCase(
		evaluationCase,
		CompletionResult{Content: reply},
		TurnPrompt{},
		"",
		"",
		"Sirens Deep",
		nil,
		principal,
	)
	if first == nil {
		t.Fatal("the gate admitted a reply that leaks both values")
	}
	_, all := ScoreEvaluationCaseAll(
		evaluationCase,
		CompletionResult{Content: reply},
		TurnPrompt{},
		"",
		"",
		"Sirens Deep",
		nil,
		principal,
	)
	if len(all) == 0 || all[0].Error() != first.Error() {
		t.Errorf("the gate's reason is not the first of the set: %q", first)
	}
}

// A clean reply reports nothing, or every assertion above would hold against a
// scorer that fails everything.
func TestACleanReplyProducesNoFailures(t *testing.T) {
	t.Parallel()
	_, failures := ScoreEvaluationCaseAll(
		EvaluationCase{ID: "clean"},
		CompletionResult{Content: "The Eco server is online with 12 players."},
		TurnPrompt{},
		"",
		"",
		"Sirens Echo",
		nil,
		Principal{Handle: "example_handle", UserID: "1024000000000000001"},
	)
	if len(failures) != 0 {
		t.Errorf("a correct reply produced %d failures: %v", len(failures), failureDetails(failures))
	}
}

// A reply can trip several forbidden patterns. Milder than the handle versus
// ID case, since the masked siblings are the same class, and the same shape.
func TestEveryForbiddenPatternMatchIsReported(t *testing.T) {
	t.Parallel()
	evaluationCase := EvaluationCase{
		ID:                "multi",
		Current:           TranscriptEntry{Content: "what does the schedule say"},
		ForbiddenPatterns: []string{"(?i)official calendar", "(?i)staff confirmed"},
	}
	if err := prepareEvaluationCase(&evaluationCase); err != nil {
		t.Fatalf("prepare: %v", err)
	}
	reply := "The official calendar says so, and staff confirmed it."
	_, failures := ScoreEvaluationCaseAll(
		evaluationCase,
		CompletionResult{Content: reply},
		TurnPrompt{},
		"",
		"",
		"Sirens Echo",
		nil,
		Principal{},
	)
	joined := strings.Join(failureDetails(failures), " | ")
	for _, pattern := range []string{"official calendar", "staff confirmed"} {
		if !strings.Contains(joined, pattern) {
			t.Errorf("pattern %q is not reported:\n%s", pattern, joined)
		}
	}
}
