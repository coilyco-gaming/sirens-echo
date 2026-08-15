package community

import (
	"context"
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
		got, err := agent.renderPhrases(context.Background(), reply)
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
	got, err := agent.renderPhrases(context.Background(), "{{phrase:no-tool}}")
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
		if _, err := agent.renderPhrases(context.Background(), reply); err == nil {
			t.Errorf("a phrase was accepted beside other text: %q", reply)
		}
	}
	// Surrounding whitespace is not other text.
	if _, err := agent.renderPhrases(context.Background(), "  {{phrase:no-tool}}\n"); err != nil {
		t.Errorf("whitespace around an invocation was read as prose: %v", err)
	}
}

// A key that resolves renders the canonical text in the harness form.
func TestAKnownKeyRendersItsPhrase(t *testing.T) {
	t.Parallel()
	got, err := phraseAgent(t).renderPhrases(context.Background(), "{{phrase:no-tool}}")
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
	got, err := phraseAgent(t).renderPhrases(context.Background(), "{{phrase:invented}}")
	if err == nil {
		t.Fatalf("an unknown key rendered: %q", got)
	}
	if strings.Contains(got, "{{phrase:") {
		t.Errorf("the marker was returned to the caller: %q", got)
	}
}

// The prompt half. It names the keys only when a registry is configured, so a
// deployment that sets no path renders the prompt it renders today.
func TestThePromptNamesTheKeysOnlyWhenConfigured(t *testing.T) {
	t.Parallel()
	const base = "You are Sirens Echo.\n"
	if got := withPhrasePolicy(base, PhraseRegistry{}); got != base {
		t.Errorf("an unconfigured registry changed the prompt:\n%q", got)
	}

	registry := PhraseRegistry{
		Schema: PhraseSchema,
		Phrases: []Phrase{
			{Key: "no-tool", Text: "no tool for that is available here"},
			{Key: "no-data", Text: "no data for that request"},
		},
	}
	got := withPhrasePolicy(base, registry)
	if !strings.HasPrefix(got, base) {
		t.Error("the policy replaced the prompt rather than appending to it")
	}
	for _, key := range []string{"no-tool", "no-data"} {
		if !strings.Contains(got, key) {
			t.Errorf("the prompt does not name the key %q", key)
		}
	}
	// The terminal rule is the one the model has to know, or it writes a phrase
	// as a prefix and the reply is refused for a reason it cannot see.
	if !strings.Contains(got, "whole reply") {
		t.Errorf("the prompt does not state the terminal rule:\n%s", got)
	}
}

// The keys are named, never the texts. A model given the text would compose
// with it rather than invoke it, which is the behaviour the registry replaces.
func TestThePromptNamesKeysNotTexts(t *testing.T) {
	t.Parallel()
	registry := PhraseRegistry{
		Schema:  PhraseSchema,
		Phrases: []Phrase{{Key: "no-tool", Text: "no tool for that is available here"}},
	}
	got := withPhrasePolicy("You are Sirens Echo.\n", registry)
	if strings.Contains(got, "no tool for that is available here") {
		t.Errorf("the prompt carries the phrase text, so the model can compose "+
			"with it rather than invoke it:\n%s", got)
	}
}
