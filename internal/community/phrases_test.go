package community

import (
	"path/filepath"
	"strings"
	"testing"
)

// A prompt rule asking for terseness is a behaviour and erodes. A key is a
// lookup and does not. See sirens-echo#176.

func trackedRegistry(t *testing.T) PhraseRegistry {
	t.Helper()
	registry, err := LoadPhraseRegistry(filepath.Join("..", "..", "agent", "phrases.yaml"))
	if err != nil {
		t.Fatalf("load tracked registry: %v", err)
	}
	return registry
}

// The tracked registry is the artifact this issue asked for, so it is checked
// rather than a fixture standing in for it.
func TestTheTrackedRegistryLoadsAndEveryPhraseRenders(t *testing.T) {
	t.Parallel()
	registry := trackedRegistry(t)
	for _, phrase := range registry.Phrases {
		rendered := harnessNotice(phrase.Text)
		if !noticeShape.MatchString(rendered) {
			t.Errorf("%s renders outside the notice shape: %q", phrase.Key, rendered)
		}
		// A phrase that changes when rendered is a phrase nobody controls: the
		// registry would say one thing and the member would read another.
		if !strings.Contains(rendered, phrase.Text) {
			t.Errorf("%s was altered by rendering: %q became %q", phrase.Key, phrase.Text, rendered)
		}
	}
}

func TestAPhraseIsInvokedByKey(t *testing.T) {
	t.Parallel()
	registry := trackedRegistry(t)
	rendered, err := registry.RenderPhrases("{{phrase:no-tool}}")
	if err != nil {
		t.Fatalf("render: %v", err)
	}
	if !noticeShape.MatchString(rendered) {
		t.Errorf("rendered = %q, want the blockquote-code form", rendered)
	}
	if strings.Contains(rendered, "{{") {
		t.Errorf("the invocation survived into the reply: %q", rendered)
	}
}

// An unknown key must not reach a member as a marker. Failing is recoverable
// through the existing repair loop; a leaked `{{phrase:typo}}` is not.
func TestAnUnknownKeyIsRefusedRatherThanLeaked(t *testing.T) {
	t.Parallel()
	registry := trackedRegistry(t)
	rendered, err := registry.RenderPhrases("{{phrase:does-not-exist}}")
	if err == nil {
		t.Fatalf("an unknown key was tolerated and rendered %q", rendered)
	}
	if !strings.Contains(err.Error(), "does-not-exist") {
		t.Errorf("the error does not name the key: %v", err)
	}
	if rendered != "" {
		t.Errorf("a failed render still returned text: %q", rendered)
	}
}

// A reply with no invocation is untouched, which is the ordinary case and the
// one that must not regress.
func TestAnOrdinaryReplyIsUntouched(t *testing.T) {
	t.Parallel()
	registry := trackedRegistry(t)
	reply := "The Eco server is online, with 4 players."
	rendered, err := registry.RenderPhrases(reply)
	if err != nil {
		t.Fatalf("render: %v", err)
	}
	if rendered != reply {
		t.Errorf("an ordinary reply was altered: %q", rendered)
	}
}

func TestSeveralInvocationsInOneReplyAllRender(t *testing.T) {
	t.Parallel()
	registry := trackedRegistry(t)
	rendered, err := registry.RenderPhrases("{{phrase:no-tool}} {{phrase:ask-narrower}}")
	if err != nil {
		t.Fatalf("render: %v", err)
	}
	for _, line := range strings.Fields(rendered) {
		if strings.Contains(line, "{{") {
			t.Errorf("an invocation survived: %q", rendered)
		}
	}
	if strings.Count(rendered, "`") != 4 {
		t.Errorf("expected two rendered phrases, got %q", rendered)
	}
}

// The validator refuses a registry that would render something a member cannot
// recognise, because the alphabet is what makes a notice identifiable.
func TestAPhraseThatCannotSurviveRenderingIsRefused(t *testing.T) {
	t.Parallel()
	bad := PhraseRegistry{
		Schema:  PhraseSchema,
		Phrases: []Phrase{{Key: "shouty", Text: "NO TOOL FOR THAT!"}},
	}
	if err := bad.validate(); err == nil {
		t.Error("a phrase outside the notice alphabet was accepted")
	}
	duplicate := PhraseRegistry{
		Schema: PhraseSchema,
		Phrases: []Phrase{
			{Key: "no-tool", Text: "no tool for that"},
			{Key: "no-tool", Text: "something else"},
		},
	}
	if err := duplicate.validate(); err == nil {
		t.Error("a duplicate key was accepted, so one phrase would shadow another")
	}
	badKey := PhraseRegistry{
		Schema:  PhraseSchema,
		Phrases: []Phrase{{Key: "No Tool", Text: "no tool for that"}},
	}
	if err := badKey.validate(); err == nil {
		t.Error("a key needing quoting was accepted")
	}
}
