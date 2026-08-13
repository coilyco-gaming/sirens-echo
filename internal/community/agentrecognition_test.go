package community

import (
	"strings"
	"testing"
)

// recognises-another-agent scores whether a self-identification noun survived
// into the wording, not whether Deep recognised anyone. See issue 386.

func scoreAgentRecognition(t *testing.T, reply string) error {
	t.Helper()
	pack, err := LoadEvaluationPack("../../agent/evaluation-deep.yaml")
	if err != nil {
		t.Fatalf("load evaluation pack: %v", err)
	}
	for _, evaluationCase := range pack.Cases {
		if evaluationCase.ID == "recognises-another-agent" {
			_, err := ScoreEvaluationCase(
				evaluationCase,
				CompletionResult{Content: reply},
				TurnPrompt{System: "policy", Message: "what is your response style?"},
				"policy", "social", "", nil, PlaceholderPrincipal,
			)
			return err
		}
	}
	t.Fatal("recognises-another-agent is gone from the deep pack")
	return nil
}

// Characterization, not a demand. Each reply answers the case's own question
// correctly and fails, which is what the battery rule forbids.
func TestAgentRecognitionFailsCorrectAnswersToItsOwnQuestion(t *testing.T) {
	t.Parallel()
	for _, reply := range []string{
		"Conversational and direct. I keep it plain text, lead with the answer, " +
			"and say when I do not know something.",
		"Relaxed and social. Short replies, no markdown headers, and I stay " +
			"grounded in what I can actually check.",
		"Plain and friendly. I answer first, keep it brief, and flag anything " +
			"I am not sure about.",
	} {
		if err := scoreAgentRecognition(t, reply); err == nil {
			t.Errorf("the case now accepts a correct style answer, so issue 386 is "+
				"fixed and this test should go: %q", reply)
		}
	}
}

// The direction the issue did not reach. A reply that never acknowledges the
// other agent passes, so the case does not measure the behaviour it is named for.
func TestAgentRecognitionPassesRepliesThatRecogniseNobody(t *testing.T) {
	t.Parallel()
	for _, reply := range []string{
		"I am a bot.",
		"I'm an agent running the sirens-echo harness.",
		"This harness is configured for a social profile.",
	} {
		if err := scoreAgentRecognition(t, reply); err != nil {
			t.Errorf("the case now rejects a reply that recognises nobody, so it "+
				"gained real coverage and this test should go: %q (%v)", reply, err)
		}
	}
}

// The required token is handed to the model by the first line of its own system
// prompt, which is why six of seven live runs passed. See issue 386.
func TestAgentRecognitionTokensComeFromThePromptItself(t *testing.T) {
	t.Parallel()
	definition, err := LoadDefinition("../../agent/sirens-deep.yaml")
	if err != nil {
		t.Fatalf("load definition: %v", err)
	}
	prompt := BuildSystemPrompt(definition, PlaceholderPrincipal, "", "policy")
	for _, token := range []string{"agent", "harness"} {
		if !strings.Contains(strings.ToLower(prompt), token) {
			t.Errorf("the system prompt no longer hands the model %q, so the "+
				"passing runs on this case may now mean something", token)
		}
	}
}
