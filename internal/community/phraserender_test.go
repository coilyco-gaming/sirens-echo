package community

import (
	"strings"
	"testing"
)

// The render half lands before the prompt half, because the reverse is what
// puts a raw marker in a member's channel. See sirens-echo#588.

func phraseAgent(t *testing.T) *Agent {
	t.Helper()
	return &Agent{
		telemetry: telemetryOrNoop(nil),
		phrases: PhraseRegistry{
			Schema:  PhraseSchema,
			Phrases: []Phrase{{Key: "no-tool", Text: "no tool for that is available here"}},
		},
	}
}

// An ordinary reply is untouched, which is every reply until something tells
// the model the syntax exists.
func TestAReplyWithNoInvocationIsUntouched(t *testing.T) {
	t.Parallel()
	const reply = "Copper is 2.4c at Kai's Emporium."
	for _, agent := range []*Agent{phraseAgent(t), {telemetry: telemetryOrNoop(nil)}} {
		got, err := agent.renderPhrases(reply)
		if err != nil {
			t.Errorf("an ordinary reply was refused: %v", err)
		}
		if got != reply {
			t.Errorf("an ordinary reply was rewritten:\n  in =%q\n  out=%q", reply, got)
		}
	}
}

// The reported risk. Without a registry the marker would reach Discord as
// literal text, so the turn fails into a canned notice instead.
func TestAnInvocationWithNoRegistryIsRefused(t *testing.T) {
	t.Parallel()
	agent := &Agent{telemetry: telemetryOrNoop(nil)}
	got, err := agent.renderPhrases("{{phrase:no-tool}}")
	if err == nil {
		t.Fatalf("a marker survived with no registry: %q", got)
	}
	if strings.Contains(got, "{{phrase:") {
		t.Errorf("the marker was returned to the caller: %q", got)
	}
}

// A phrase is the whole reply or it is not a phrase. A prefix returns every
// padding problem the registry exists to prevent.
func TestAnInvocationMustBeTheWholeReply(t *testing.T) {
	t.Parallel()
	agent := phraseAgent(t)
	for _, reply := range []string{
		"Sorry, {{phrase:no-tool}}",
		"{{phrase:no-tool}} but I could look it up another way.",
		"{{phrase:no-tool}} {{phrase:no-tool}} extra",
	} {
		if _, err := agent.renderPhrases(reply); err == nil {
			t.Errorf("a phrase was accepted beside other text: %q", reply)
		}
	}
	// Surrounding whitespace is not other text.
	if _, err := agent.renderPhrases("  {{phrase:no-tool}}\n"); err != nil {
		t.Errorf("whitespace around an invocation was read as prose: %v", err)
	}
}

// A key that resolves renders the canonical text in the harness form.
func TestAKnownKeyRendersItsPhrase(t *testing.T) {
	t.Parallel()
	got, err := phraseAgent(t).renderPhrases("{{phrase:no-tool}}")
	if err != nil {
		t.Fatalf("a known key was refused: %v", err)
	}
	if !strings.Contains(got, "no tool for that is available here") {
		t.Errorf("the phrase was not rendered: %q", got)
	}
	if strings.Contains(got, "{{phrase:") {
		t.Errorf("the marker survived rendering: %q", got)
	}
}

// An unknown key fails the turn, which reaches the member as a canned notice
// rather than as model prose. See sirens-echo#176.
func TestAnUnknownKeyIsRefused(t *testing.T) {
	t.Parallel()
	got, err := phraseAgent(t).renderPhrases("{{phrase:invented}}")
	if err == nil {
		t.Fatalf("an unknown key rendered: %q", got)
	}
	if strings.Contains(got, "{{phrase:") {
		t.Errorf("the marker was returned to the caller: %q", got)
	}
}
