package community

import (
	"strings"
	"testing"
)

// A client holding several servers has to tell an agent from a data source.
// See sirens-echo#647.

func TestOwnInstructionsNameTheDeployment(t *testing.T) {
	t.Parallel()
	agent := &Agent{cfg: Config{Definition: Definition{Identity: "Sirens Echo"}}}
	text := agent.serverInstructions()
	if !strings.Contains(text, "Sirens Echo") {
		t.Errorf("instructions do not name the deployment:\n%s", text)
	}
	// Deep and Echo are the same binary with different definitions, so the
	// text has to differ between them or it distinguishes nothing.
	deep := &Agent{cfg: Config{Definition: Definition{Identity: "Sirens Deep of Coilyco"}}}
	if deep.serverInstructions() == text {
		t.Error("two deployments published identical instructions")
	}
}

// A definition with no identity still produces a sentence rather than one with
// a hole in it.
func TestOwnInstructionsSurviveAnEmptyIdentity(t *testing.T) {
	t.Parallel()
	agent := &Agent{cfg: Config{Definition: Definition{}}}
	text := agent.serverInstructions()
	if strings.Contains(text, "  ") || strings.Contains(text, "One conversational turn with ,") {
		t.Errorf("an empty identity left a hole in the sentence:\n%s", text)
	}
	if strings.TrimSpace(text) == "" {
		t.Error("an empty identity produced no instructions")
	}
}

// The point is that it is an agent, not a lookup. A client that mistakes it
// for a data source will ask it the wrong questions.
func TestOwnInstructionsSayItIsAnAgent(t *testing.T) {
	t.Parallel()
	agent := &Agent{cfg: Config{Definition: Definition{Identity: "Sirens Echo"}}}
	text := strings.ToLower(agent.serverInstructions())
	for _, want := range []string{"agent", "decline", "passthrough"} {
		if !strings.Contains(text, want) {
			t.Errorf("instructions never mention %q:\n%s", want, text)
		}
	}
}

// A consumer inlines this on every turn, so it is a permanent per-turn cost.
func TestOwnInstructionsStayShort(t *testing.T) {
	t.Parallel()
	agent := &Agent{cfg: Config{Definition: Definition{Identity: "Sirens Echo"}}}
	if got := len(agent.serverInstructions()); got > 1024 {
		t.Errorf("instructions are %d bytes, too long to carry every turn", got)
	}
}
