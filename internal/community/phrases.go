package community

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"regexp"
	"strings"

	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/trace"
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

// Configured reports whether a registry was loaded. An empty one renders
// nothing, which is the deployment that names no path.
func (r PhraseRegistry) Configured() bool { return len(r.Phrases) > 0 }

// Invoked reports whether a reply carries an invocation at all, so the reply
// path can leave an ordinary reply untouched.
func Invoked(reply string) bool { return phraseInvocation.MatchString(reply) }

// InvokedKeys names the keys a reply invoked, in order. The registry authors
// these, so a caller may use one as a metric label. See sirens-echo#176.
func InvokedKeys(reply string) []string {
	matches := phraseInvocation.FindAllStringSubmatch(reply, -1)
	keys := make([]string, 0, len(matches))
	for _, match := range matches {
		keys = append(keys, strings.TrimSpace(match[1]))
	}
	return keys
}

// Terminal reports whether one invocation is the whole reply. A prefix returns
// every padding problem the registry exists to prevent. See sirens-echo#176.
func Terminal(reply string) bool {
	// Exactly one. Stripping them all and finding nothing left was also true of
	// two, and two phrases is not a phrase. See sirens-echo#613.
	if len(phraseInvocation.FindAllStringIndex(reply, -1)) != 1 {
		return false
	}
	return strings.TrimSpace(phraseInvocation.ReplaceAllString(reply, "")) == ""
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

// renderPhrases resolves an invocation the model wrote. A reply carrying none
// is returned untouched, which is every reply until the prompt names the keys.
func (a *Agent) renderPhrases(ctx context.Context, reply string) (string, error) {
	if !Invoked(reply) {
		return reply, nil
	}
	if !a.phrases.Configured() {
		return "", fmt.Errorf("reply invokes a phrase and no registry is configured")
	}
	if !Terminal(reply) {
		return "", fmt.Errorf("reply invokes a phrase alongside other text")
	}
	rendered, err := a.phrases.RenderPhrases(reply)
	if err != nil {
		return "", err
	}
	// Counted per key, which is the signal sirens-echo#176 asked for: which
	// boundaries members actually probe, and how often.
	for _, key := range InvokedKeys(reply) {
		a.telemetry.RecordPhrase(ctx, key)
		a.telemetry.Info(ctx, "response.phrase.invoked", slog.String("phrase.key", key))
		trace.SpanFromContext(ctx).SetAttributes(attribute.String("response.phrase", key))
	}
	return rendered, nil
}

// evaluationPhrases loads the registry from the same variable a deployment
// reads, so an eval measures the prompt the service runs. See #176.
func evaluationPhrases() (PhraseRegistry, error) {
	path := strings.TrimSpace(os.Getenv("SIRENS_ECHO_PHRASES"))
	if path == "" {
		return PhraseRegistry{}, nil
	}
	return LoadPhraseRegistry(path)
}

// evaluationSystemPrompt is the deployed prompt, phrase policy included. The
// eval paths built it without one, so they scored a prompt nothing renders.
func evaluationSystemPrompt(
	definition Definition,
	principal Principal,
	composed, localSkillpack string,
) (string, error) {
	phrases, err := evaluationPhrases()
	if err != nil {
		return "", err
	}
	return withPhrasePolicy(
		BuildSystemPrompt(definition, principal, composed, localSkillpack),
		phrases,
	), nil
}

// withPhrasePolicy names the keys a reply may invoke. A prompt with no registry
// is returned unchanged, which is the deployment that renders nothing.
func withPhrasePolicy(prompt string, registry PhraseRegistry) string {
	if !registry.Configured() {
		return prompt
	}
	return prompt + "\n" + fmt.Sprintf(
		`Some answers are canonical phrases rather than prose. Invoke one by writing
{{phrase:key}} and nothing else, because an invocation is the whole reply and a
phrase beside other text is refused. Use one only when it answers exactly, and
answer normally otherwise. Available keys: %s.`,
		strings.Join(registry.Keys(), ", "),
	) + "\n"
}
