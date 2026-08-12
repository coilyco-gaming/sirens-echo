package community

import (
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
)

// approvedComposedSkills is duplicated on purpose, so widening the reviewed set
// changes a test rather than only a config file. See docs/sirens-echo-compose.md.
var approvedComposedSkills = map[string]struct{}{
	"tooling-discord-community-host":          {},
	"tooling-customer-success-signal-routing": {},
	"tooling-customer-success-trust-repair":   {},
	"writing-kai-voice":                       {},
	"writing-social-cultural-reading":         {},
	"writing-social-editorial-loop":           {},
	"writing-social-trust-boundaries":         {},
	"writing-system-improvement-vocab":        {},
	"writing-voice-adaptation":                {},
	"writing-voice-guide-linter":              {},
	"personal-preference-colors":              {},
	"personal-preference-games":               {},
	"personal-preference-animals":             {},
	"personal-preference-shows":               {},
	"personal-preference-anime":               {},
	"personal-preference-books":               {},
	"personal-preference-movies":              {},
	"personal-preference-fabrication":         {},
	"personal-preferences":                    {},
}

// deniedComposedSkills must never compose into an agent that answers
// strangers. Every one of these stays in the private catalogue.
var deniedComposedSkills = map[string]string{
	"kai-career":                 "private career context",
	"kai-job-search":             "private job search",
	"kai-grill-me":               "private operating context",
	"kai-collaboration":          "private collaboration context",
	"kai-kapwing-pr-review":      "employer team and domain context",
	"kai-linkedin-voice":         "a member's personal channel voice",
	"kai-linkedin-video":         "a member's personal channel format",
	"kai-bio-surface":            "resume and identity surface, points at private lore",
	"kai-public-repos":           "names private sibling repositories",
	"kai-engineering-voice":      "code review and eng-channel posts, not this agent",
	"kai-design-language":        "art direction, and low-context: required",
	"personal-preference-social": "an organization cannot own a person's social accounts",
	"tooling-cross-repo-infra":   "fleet mutation surface",
}

var (
	declaredSkill = regexp.MustCompile(`^\s*skill\s+"([^"]+)"\s+path="([^"]+)"`)
	requestSource = regexp.MustCompile(`^\s*source\s+"([^"]+)"\s+(\w+)="([^"]+)"`)
)

func readCompose(t *testing.T, name string) string {
	t.Helper()
	path := filepath.Join("..", "..", "agent", "compose", name)
	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read %s: %v", path, err)
	}
	return string(raw)
}

// The declaration is the allowlist. A name outside the reviewed set reaching an
// agent that answers strangers is the failure this whole mechanism exists for.
func TestDeclarationAdmitsOnlyReviewedSources(t *testing.T) {
	t.Parallel()
	seen := map[string]struct{}{}
	for _, line := range strings.Split(readCompose(t, "aos-public.kdl"), "\n") {
		match := declaredSkill.FindStringSubmatch(line)
		if match == nil {
			continue
		}
		name, path := match[1], match[2]
		if _, approved := approvedComposedSkills[name]; !approved {
			t.Errorf("declaration admits unreviewed source %q", name)
		}
		if reason, denied := deniedComposedSkills[name]; denied {
			t.Errorf("declaration admits denied source %q: %s", name, reason)
		}
		if path != "skills/"+name {
			t.Errorf("source %q declares path %q, want skills/%s", name, path, name)
		}
		if _, duplicate := seen[name]; duplicate {
			t.Errorf("declaration names %q twice", name)
		}
		seen[name] = struct{}{}
	}
	if len(seen) == 0 {
		t.Fatal("declaration admits nothing")
	}
}

// A glob would reintroduce exactly the accident the declaration form prevents,
// since a pattern silently widens when the upstream catalogue grows.
func TestDeclarationNamesNoGlobs(t *testing.T) {
	t.Parallel()
	for _, line := range strings.Split(readCompose(t, "aos-public.kdl"), "\n") {
		if match := declaredSkill.FindStringSubmatch(line); match != nil {
			if strings.ContainsAny(match[1], "*?[") {
				t.Errorf("declaration uses a glob %q; enumerate instead", match[1])
			}
		}
	}
}

// root= hands selection to the source repository's own roles.kdl, which for the
// private catalogue deliberately binds Kai's career and LinkedIn context.
func TestRequestSourcesAlwaysUseADeclaration(t *testing.T) {
	t.Parallel()
	sources := 0
	for _, line := range strings.Split(readCompose(t, "request.kdl"), "\n") {
		if strings.HasPrefix(strings.TrimSpace(line), "//") {
			continue
		}
		match := requestSource.FindStringSubmatch(line)
		if match == nil {
			continue
		}
		sources++
		if match[2] != "declaration" {
			t.Errorf("source %q uses %s=, want declaration=", match[1], match[2])
		}
	}
	if sources == 0 {
		t.Fatal("request names no source")
	}
}

// A composing profile must show the bundle, and the neutral profile must not.
// See docs/sirens-echo-compose.md for why the anchors changed.
func TestComposedProfileRequiresItsBundleSurface(t *testing.T) {
	t.Parallel()
	definition := Definition{
		Identity:      "Sirens Deep of Coilyco",
		AuditRole:     "general",
		ResponseStyle: ResponseStyleSocial,
		Composed:      true,
	}
	composed, err := LoadBundle(filepath.Join(writeFixtureBundle(t), "creator"))
	if err != nil {
		t.Fatalf("LoadBundle: %v", err)
	}
	prompt := BuildSystemPrompt(definition, PlaceholderPrincipal, composed, "general CoilyCo policy")
	for _, expected := range []string{
		"<composed-identity>",
		"Agent-compose assigned the",
		"## Personality meld",
		"**Role skill //",
		"CoilyCo house style",
	} {
		if !strings.Contains(prompt, expected) {
			t.Fatalf("composed prompt missing %q", expected)
		}
	}
	if err := ValidateSystemPrompt(definition, PlaceholderPrincipal, prompt); err != nil {
		t.Fatalf("ValidateSystemPrompt: %v", err)
	}
	// A bundle that failed to compose must stop startup, not answer neutrally.
	bare := BuildSystemPrompt(definition, PlaceholderPrincipal, "", "general CoilyCo policy")
	if err := ValidateSystemPrompt(definition, PlaceholderPrincipal, bare); err == nil {
		t.Fatal("validator accepted a composing profile with no bundle")
	}
	// And the neutral profile must never carry one.
	neutral := Definition{Identity: "Sirens Echo", AuditRole: "community", ResponseStyle: ResponseStyleNeutral}
	leaked := BuildSystemPrompt(neutral, PlaceholderPrincipal, composed, "approved Sirens facts")
	if err := ValidateSystemPrompt(neutral, PlaceholderPrincipal, leaked); err == nil {
		t.Fatal("validator accepted a bundle in the neutral profile")
	}
}

// writeFixtureBundle builds a minimal materialized bundle. Generating beats
// tracking: the tree is Markdown, which documentation-layout bounds to docs.
func writeFixtureBundle(t *testing.T) string {
	t.Helper()
	bundles := t.TempDir()
	skill := filepath.Join(bundles, "creator", "content", "skills", "aos-public", "writing-kai-voice")
	if err := os.MkdirAll(skill, 0o755); err != nil {
		t.Fatalf("MkdirAll: %v", err)
	}
	for path, body := range map[string]string{
		filepath.Join(bundles, "creator", "manifest.json"):              `{"format":"agent-compose.bundle.v0.1","role":"creator"}`,
		filepath.Join(bundles, "creator", "content", "instructions.md"): fixtureCard,
		filepath.Join(skill, "SKILL.md"):                                fixtureSkill,
	} {
		if err := os.WriteFile(path, []byte(body), 0o644); err != nil {
			t.Fatalf("WriteFile %s: %v", path, err)
		}
	}
	return bundles
}

const fixtureCard = `# Role instructions

Agent-compose assigned the ` + "`creator`" + ` role from the caller's compose request.

**Role skill // ` + "`role-creator`" + `**
**Agent // Gem (they)**

## Personality meld

Fixture meld for the offline harness.
`

const fixtureSkill = `---
name: writing-kai-voice
---

# CoilyCo house style

Fixture body.
`
