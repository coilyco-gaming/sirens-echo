package community

import (
	"fmt"
	"os"
	"regexp"
	"strings"

	"gopkg.in/yaml.v3"
)

// A registry the model invokes by key. See docs/sirens-echo-phrases.md.

// PhraseSchema is the registry's contract.
const PhraseSchema = "sirens-discord-ops.phrases.v1"

// phraseKeyPattern is the key shape. Lowercase and hyphenated, so a key never
// needs quoting and never collides with prose.
var phraseKeyPattern = regexp.MustCompile(`^[a-z][a-z0-9-]{1,38}[a-z0-9]$`)

// phraseInvocation is what the model writes to invoke one.
var phraseInvocation = regexp.MustCompile(`\{\{phrase:([^}]*)\}\}`)

// Phrase is one canonical string.
type Phrase struct {
	Key  string `yaml:"key"`
	Text string `yaml:"text"`
}

// PhraseRegistry is the source-controlled phrase list.
type PhraseRegistry struct {
	Schema  string   `yaml:"schema"`
	Phrases []Phrase `yaml:"phrases"`
}

// LoadPhraseRegistry reads and validates the registry.
func LoadPhraseRegistry(path string) (PhraseRegistry, error) {
	raw, err := os.ReadFile(path)
	if err != nil {
		return PhraseRegistry{}, fmt.Errorf("phrase registry: %w", err)
	}
	var registry PhraseRegistry
	if err := yaml.Unmarshal(raw, &registry); err != nil {
		return PhraseRegistry{}, fmt.Errorf("phrase registry: %w", err)
	}
	return registry, registry.validate()
}

// validate refuses a registry that would render something a member cannot
// recognise as harness output. See docs/sirens-echo-phrases.md.
func (r PhraseRegistry) validate() error {
	if r.Schema != PhraseSchema {
		return fmt.Errorf("phrase registry: schema is %q, want %q", r.Schema, PhraseSchema)
	}
	if len(r.Phrases) == 0 {
		return fmt.Errorf("phrase registry: no phrases")
	}
	seen := map[string]bool{}
	for _, phrase := range r.Phrases {
		if !phraseKeyPattern.MatchString(phrase.Key) {
			return fmt.Errorf("phrase registry: key %q is not lowercase and hyphenated", phrase.Key)
		}
		if seen[phrase.Key] {
			return fmt.Errorf("phrase registry: key %q appears twice", phrase.Key)
		}
		seen[phrase.Key] = true
		// A phrase that changes when rendered is a phrase nobody controls.
		if noticeBody(phrase.Text) != strings.TrimSpace(phrase.Text) {
			return fmt.Errorf("phrase registry: %q does not survive the notice alphabet", phrase.Key)
		}
	}
	return nil
}

// Lookup answers by key.
func (r PhraseRegistry) Lookup(key string) (Phrase, bool) {
	for _, phrase := range r.Phrases {
		if phrase.Key == key {
			return phrase, true
		}
	}
	return Phrase{}, false
}

// Keys lists every key, for the prompt that tells the model what it may invoke.
func (r PhraseRegistry) Keys() []string {
	keys := make([]string, 0, len(r.Phrases))
	for _, phrase := range r.Phrases {
		keys = append(keys, phrase.Key)
	}
	return keys
}

// RenderPhrases replaces every invocation with its rendered phrase. An unknown
// key is an error rather than a marker a member reads.
func (r PhraseRegistry) RenderPhrases(reply string) (string, error) {
	var unknown []string
	rendered := phraseInvocation.ReplaceAllStringFunc(reply, func(match string) string {
		key := strings.TrimSpace(phraseInvocation.FindStringSubmatch(match)[1])
		phrase, known := r.Lookup(key)
		if !known {
			unknown = append(unknown, key)
			return match
		}
		return harnessNotice(phrase.Text)
	})
	if len(unknown) > 0 {
		return "", fmt.Errorf("reply invokes unknown phrase %s", strings.Join(unknown, ", "))
	}
	return rendered, nil
}
