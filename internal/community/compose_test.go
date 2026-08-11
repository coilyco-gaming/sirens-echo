package community

import (
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
)

// approvedComposedPatterns is duplicated on purpose, so widening the allowlist
// changes a test rather than only a config file. See docs/sirens-echo-compose.md.
var approvedComposedPatterns = map[string]struct{}{
	"personal-preference-*":          {},
	"writing-kai-voice":              {},
	"writing-social-*":               {},
	"writing-voice-adaptation":       {},
	"tooling-discord-community-host": {},
}

// deniedComposedSkills must never compose into an agent that answers
// strangers. A pattern that would match one of these fails the suite.
var deniedComposedSkills = map[string]string{
	"kai-career":               "private career context",
	"kai-job-search":           "private job search",
	"kai-grill-me":             "private operating context",
	"kai-collaboration":        "private collaboration context",
	"kai-kapwing-pr-review":    "employer team and domain context",
	"writing-kai-linkedin":     "a member's personal channel voice",
	"tooling-cross-repo-infra": "fleet mutation surface",
}

func composedSkillsIn(t *testing.T) (string, []string) {
	t.Helper()
	path := filepath.Join("..", "..", "agent", "compose", "roles.kdl")
	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read %s: %v", path, err)
	}
	body := string(raw)
	var found []string
	for _, line := range strings.Split(body, "\n") {
		trimmed := strings.TrimSpace(line)
		if strings.HasPrefix(trimmed, "//") {
			continue
		}
		if m := regexp.MustCompile(`composed-skill\s+"([^"]+)"`).FindStringSubmatch(trimmed); m != nil {
			found = append(found, m[1])
		}
	}
	return body, found
}

func TestComposeAllowlistHoldsOnlyApprovedSources(t *testing.T) {
	t.Parallel()
	_, found := composedSkillsIn(t)
	if len(found) == 0 {
		t.Fatal("no composed-skill entries parsed, so the allowlist proves nothing")
	}
	for _, name := range found {
		if _, ok := approvedComposedPatterns[name]; !ok {
			t.Errorf("composed-skill %q is not on the reviewed allowlist", name)
		}
	}
}

// Globs are allowed, so the invariant is that no pattern reaches a denied
// source. See docs/sirens-echo-compose.md.
func TestComposePatternsNeverReachDeniedSources(t *testing.T) {
	t.Parallel()
	_, found := composedSkillsIn(t)
	for _, pattern := range found {
		for denied, reason := range deniedComposedSkills {
			matched, err := filepath.Match(pattern, denied)
			if err != nil {
				t.Fatalf("composed-skill %q is not a valid pattern: %v", pattern, err)
			}
			if matched {
				t.Errorf("composed-skill %q reaches %q, which must never compose here: %s",
					pattern, denied, reason)
			}
		}
	}
}

// The host profile declares `global profile` and `global lore`. Either one here
// would put private operating context into an agent that answers strangers.
func TestComposeDeclaresNoGlobalRepositories(t *testing.T) {
	t.Parallel()
	body, _ := composedSkillsIn(t)
	for _, line := range strings.Split(body, "\n") {
		trimmed := strings.TrimSpace(line)
		if strings.HasPrefix(trimmed, "//") {
			continue
		}
		if regexp.MustCompile(`^global\s`).MatchString(trimmed) {
			t.Errorf("global repository declared: %q", trimmed)
		}
	}
}
