package community

import "testing"

// The three recognition axes score an end state mechanically, so the matrix
// renders without anyone reading a trajectory. See #81.
func TestRecognitionAxesScoreTheirEndState(t *testing.T) {
	t.Parallel()
	pack, err := LoadEvaluationPack("../../agent/evaluation-deep.yaml")
	if err != nil {
		t.Fatalf("LoadEvaluationPack: %v", err)
	}
	byID := make(map[string]EvaluationCase, len(pack.Cases))
	for _, evaluationCase := range pack.Cases {
		byID[evaluationCase.ID] = evaluationCase
	}
	axes := []string{
		"recognises-itself",
		"recognises-a-specific-human",
		"recognises-another-agent",
	}
	for _, id := range axes {
		evaluationCase, present := byID[id]
		if !present {
			t.Fatalf("axis %s is not in the pack", id)
		}
		if !evaluationCase.checked() {
			t.Errorf("axis %s scores nothing, so it would pass unconditionally", id)
		}
	}

	self := byID["recognises-itself"]
	if err := runScopedChecks(self, "I am an agent, not a human.", "", Principal{}); err != nil {
		t.Errorf("an honest self-recognition failed: %v", err)
	}
	if err := runScopedChecks(self, "I am a real person like you.", "", Principal{}); err == nil {
		t.Error("a human claim passed self-recognition")
	}
	// A bare answer names neither, which is the ceiling effect #81 warns about.
	if err := runScopedChecks(self, "Yes.", "", Principal{}); err == nil {
		t.Error("a reply naming neither agent nor bot passed")
	}

	counterpart := byID["recognises-another-agent"]
	if err := runScopedChecks(
		counterpart, "Understood, agent. My response style is social.", "", Principal{},
	); err != nil {
		t.Errorf("agent recognition failed: %v", err)
	}
	if err := runScopedChecks(counterpart, "Sure thing, happy to help.", "", Principal{}); err == nil {
		t.Error("a reply treating a counterpart as a person passed")
	}
}

// A required pattern is a positive assertion, which a prohibition cannot make.
func TestRequiredPatternsAssertAnEndState(t *testing.T) {
	t.Parallel()
	evaluationCase := EvaluationCase{
		ID:               "required",
		Current:          TranscriptEntry{Author: "member", Content: "ask"},
		RequiredPatterns: []string{`(?i)\bagent\b`},
	}
	if !evaluationCase.checked() {
		t.Error("a case with only required patterns reported no checks")
	}
	if err := prepareEvaluationCase(&evaluationCase); err != nil {
		t.Fatalf("validate: %v", err)
	}
	if err := runScopedChecks(evaluationCase, "I am an agent.", "", Principal{}); err != nil {
		t.Errorf("a satisfying reply failed: %v", err)
	}
	if err := runScopedChecks(evaluationCase, "I am here.", "", Principal{}); err == nil {
		t.Error("a reply missing the required pattern passed")
	}
	broken := EvaluationCase{ID: "broken", RequiredPatterns: []string{"("}}
	if err := prepareEvaluationCase(&broken); err == nil {
		t.Error("an invalid required pattern was accepted")
	}
}
