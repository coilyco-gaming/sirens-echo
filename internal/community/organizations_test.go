package community

import (
	"path/filepath"
	"strings"
	"testing"
)

// The org relationship is one source both agents read, so neither infers it.
// See sirens-echo#806 and docs/sirens-echo-organizations.md.

const orgSkillRoot = ".agents/skills/coilyco-org"

// profileDefinitions is the two shipped profiles, which is what "check both,
// not one" in the acceptance means.
func profileDefinitions() map[string]string {
	return map[string]string{
		"echo": filepath.Join("..", "..", "agent", "sirens-echo.yaml"),
		"deep": filepath.Join("..", "..", "agent", "sirens-deep.yaml"),
	}
}

// The acceptance criterion: both rendered bundles include the source. A fact
// only one agent carries is the drift this issue exists to prevent.
func TestBothProfilesComposeTheOrgSource(t *testing.T) {
	t.Parallel()
	for name, path := range profileDefinitions() {
		definition, err := LoadDefinition(path)
		if err != nil {
			t.Fatalf("load %s: %v", name, err)
		}
		roots := strings.Join(definition.LocalSkillRoots, " ")
		if !strings.Contains(roots, orgSkillRoot) {
			t.Errorf("%s composes no org source, so it answers from inference: %s",
				name, roots)
		}
	}
}

// Rendered rather than declared, because a root named in a definition and a
// root that reached the prompt are different claims.
func TestTheOrgFactsReachBothPrompts(t *testing.T) {
	t.Parallel()
	for name, path := range profileDefinitions() {
		definition, err := LoadDefinition(path)
		if err != nil {
			t.Fatalf("load %s: %v", name, err)
		}
		pack, err := LoadSkillpack(prefixedRoots(definition.LocalSkillRoots))
		if err != nil {
			t.Fatalf("load %s skillpack: %v", name, err)
		}
		for _, fact := range []string{
			"Sirens Discord",
			"Coilyco Gaming",
			"Robotics Division",
			"who do you work for",
		} {
			if !strings.Contains(pack, fact) {
				t.Errorf("%s's policy does not carry %q", name, fact)
			}
		}
	}
}

// One statement, not two that happen to agree today. The two profiles share no
// other local root, so a copy edit to one would otherwise silently split them.
func TestBothProfilesReadTheSameOrgText(t *testing.T) {
	t.Parallel()
	rendered := map[string]string{}
	for name, path := range profileDefinitions() {
		definition, err := LoadDefinition(path)
		if err != nil {
			t.Fatalf("load %s: %v", name, err)
		}
		for _, root := range definition.LocalSkillRoots {
			if !strings.Contains(root, orgSkillRoot) {
				continue
			}
			pack, err := LoadSkillpack([]string{filepath.Join("..", "..", root)})
			if err != nil {
				t.Fatalf("load %s org source: %v", name, err)
			}
			rendered[name] = pack
		}
	}
	if len(rendered) != 2 {
		t.Fatalf("resolved %d org sources, want one per profile", len(rendered))
	}
	if rendered["echo"] != rendered["deep"] {
		t.Error("the profiles read different org text, so they can drift apart")
	}
}

// The constraint is public-safe copy, and a bounded reference is the whole
// point. A statement that hedges is one an agent will improvise around.
func TestTheOrgSourceKeepsTheOrganizationsApart(t *testing.T) {
	t.Parallel()
	pack, err := LoadSkillpack([]string{filepath.Join("..", "..", orgSkillRoot)})
	if err != nil {
		t.Fatalf("load org source: %v", err)
	}
	if !strings.Contains(pack, "never employed by Coilyco Gaming") {
		t.Error("the source does not say staff are not Coilyco Gaming employees")
	}
	if !strings.Contains(pack, "never a member of Sirens Discord staff") {
		t.Error("the source does not say the agent is not community staff")
	}
	// Bounds rather than a private detail. The terms are not ours to describe.
	if !strings.Contains(pack, "not something to disclose") {
		t.Error("the source does not bound what may be said about the contract")
	}
}

// prefixedRoots resolves a definition's roots from this package's directory,
// which is two levels below the repository root.
func prefixedRoots(roots []string) []string {
	resolved := make([]string, 0, len(roots))
	for _, root := range roots {
		resolved = append(resolved, filepath.Join("..", "..", root))
	}
	return resolved
}
