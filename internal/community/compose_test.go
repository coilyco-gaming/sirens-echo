package community

import (
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
)

// approvedComposedSkills is duplicated on purpose, so widening the allowlist
// changes a test rather than only a config file. See docs/sirens-echo-compose.md.
var approvedComposedSkills = map[string]struct{}{
	"personal-preference-colors":      {},
	"personal-preference-animals":     {},
	"personal-preference-games":       {},
	"personal-preference-shows":       {},
	"personal-preference-anime":       {},
	"personal-preference-books":       {},
	"personal-preference-movies":      {},
	"personal-preference-fabrication": {},
	"writing-kai-voice":               {},
}

// deniedComposedSkills are sources that must never compose into an agent that
// answers strangers, each for a stated reason.
var deniedComposedSkills = map[string]string{
	"personal-preference-social": "a person's social accounts are not an organization's taste",
	"kai-career":                 "private career context",
	"kai-job-search":             "private job search",
	"kai-grill-me":               "private operating context",
	"kai-collaboration":          "private collaboration context",
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
		if _, ok := approvedComposedSkills[name]; !ok {
			t.Errorf("composed-skill %q is not on the reviewed allowlist", name)
		}
		if reason, denied := deniedComposedSkills[name]; denied {
			t.Errorf("composed-skill %q must never compose here: %s", name, reason)
		}
	}
}

// A glob is the failure mode this allowlist exists to prevent:
// personal-preference-* silently includes personal-preference-social.
func TestComposeAllowlistUsesExactNames(t *testing.T) {
	t.Parallel()
	_, found := composedSkillsIn(t)
	for _, name := range found {
		if strings.ContainsAny(name, "*?[") {
			t.Errorf("composed-skill %q is a glob; list sources by exact name", name)
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
