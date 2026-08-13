package community

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// The deny list is what keeps private context away from an agent that answers
// strangers. Enforcement is a property of names, so it needs no real catalogue.

// fixtureCatalog writes a catalogue holding exactly the named skills, so a test
// controls the whole target set rather than inheriting one.
func fixtureCatalog(t *testing.T, names ...string) string {
	t.Helper()
	root := t.TempDir()
	for _, name := range names {
		if err := os.MkdirAll(filepath.Join(root, ".agents", "composed", name), 0o755); err != nil {
			t.Fatalf("MkdirAll %s: %v", name, err)
		}
	}
	return root
}

// Naming a denied source exactly is an error rather than a silent drop, and the
// whole map is the target set so a new entry is covered on arrival.
func TestEveryDeniedSkillIsRefusedWhenNamedExactly(t *testing.T) {
	t.Parallel()
	if len(DeniedComposedSkills) == 0 {
		t.Fatal("the deny list is empty, so this test asserts nothing")
	}
	for denied, reason := range DeniedComposedSkills {
		catalog := fixtureCatalog(t, denied, "writing-house-style")
		graph := RoleGraph{Patterns: map[string][]string{"creator": {denied}}}
		admitted, _, err := ExpandRoleWithExclusions([]string{catalog}, "creator", graph)
		if err == nil {
			t.Errorf("%q was accepted when named exactly", denied)
			continue
		}
		if _, leaked := admitted[denied]; leaked {
			t.Errorf("%q was admitted alongside its own error", denied)
		}
		if !strings.Contains(err.Error(), reason) {
			t.Errorf("%q: %v, expected the error to carry the reason %q", denied, err, reason)
		}
	}
}

// A glob brushing past a denied member drops it and keeps the rest, which is
// what lets the graph stay globbed against a catalogue it does not control.
func TestFamilyGlobDropsTheDeniedMemberAndKeepsTheRest(t *testing.T) {
	t.Parallel()
	catalog := fixtureCatalog(t,
		"personal-preference-social",
		"personal-preference-writing",
		"personal-preference-review",
	)
	graph := RoleGraph{Patterns: map[string][]string{"creator": {"personal-preference-*"}}}
	admitted, dropped, err := ExpandRoleWithExclusions([]string{catalog}, "creator", graph)
	if err != nil {
		t.Fatalf("a family glob failed on a denied member: %v", err)
	}
	if _, leaked := admitted["personal-preference-social"]; leaked {
		t.Error("the denied member survived a family glob")
	}
	for _, kept := range []string{"personal-preference-writing", "personal-preference-review"} {
		if _, ok := admitted[kept]; !ok {
			t.Errorf("%q was dropped along with the denied member", kept)
		}
	}
	if len(dropped) != 1 || !strings.Contains(dropped[0], "personal-preference-social") {
		t.Errorf("dropped = %v, want the denied member reported", dropped)
	}
}

// A wide glob over the private family must be refused rather than quietly
// yielding nothing, because silence reads the same as an empty catalogue.
func TestAWideGlobOverThePrivateFamilyAdmitsNothing(t *testing.T) {
	t.Parallel()
	catalog := fixtureCatalog(t, "kai-career", "kai-job-search", "kai-grill-me")
	graph := RoleGraph{Patterns: map[string][]string{"creator": {"kai-*"}}}
	admitted, dropped, err := ExpandRoleWithExclusions([]string{catalog}, "creator", graph)
	if err != nil {
		t.Fatalf("expansion errored: %v", err)
	}
	// Characterization. Every match was denied, so the pattern contributes
	// nothing and does not trip the matches-nothing guard.
	if len(admitted) != 0 {
		t.Errorf("admitted %v from a catalogue holding only denied sources", SortedNames(admitted))
	}
	if len(dropped) != 3 {
		t.Errorf("dropped %d of 3 denied sources; the refusal must be reported", len(dropped))
	}
}

// An empty selector hides a rename, so it fails the build rather than shrinking
// the bundle to whatever still happens to match.
func TestExpansionRefusesAPatternThatMatchesNothing(t *testing.T) {
	t.Parallel()
	catalog := fixtureCatalog(t, "writing-house-style")
	graph := RoleGraph{Patterns: map[string][]string{"creator": {"writing-nothing-*"}}}
	if _, err := ExpandRole([]string{catalog}, "creator", graph); err == nil {
		t.Fatal("a pattern matching nothing was accepted")
	}
}

// Two catalogues owning one name is ambiguous. First-wins would decide which
// copy of a skill an agent gets by directory order.
func TestExpansionRefusesASkillOwnedByTwoCatalogues(t *testing.T) {
	t.Parallel()
	first := fixtureCatalog(t, "writing-house-style")
	second := fixtureCatalog(t, "writing-house-style")
	_, _, err := ExpandRoleWithExclusions([]string{first, second}, "creator",
		RoleGraph{Patterns: map[string][]string{"creator": {"writing-*"}}})
	if err == nil {
		t.Fatal("the same skill was accepted from two catalogues")
	}
	if !strings.Contains(err.Error(), "one catalogue must own it") {
		t.Errorf("%v, expected the error to name the clash", err)
	}
}

// Expansion reports where each admitted name came from, which is what lets a
// wider compile show its sources instead of asserting them.
func TestExpansionReportsTheOwningCatalogue(t *testing.T) {
	t.Parallel()
	first := fixtureCatalog(t, "writing-house-style")
	second := fixtureCatalog(t, "tooling-discord-community-host")
	admitted, _, err := ExpandRoleWithExclusions([]string{first, second}, "creator",
		RoleGraph{Patterns: map[string][]string{"creator": {"writing-*", "tooling-*"}}})
	if err != nil {
		t.Fatalf("expansion errored: %v", err)
	}
	if admitted["writing-house-style"] != first {
		t.Errorf("writing-house-style came from %q, want %q", admitted["writing-house-style"], first)
	}
	if admitted["tooling-discord-community-host"] != second {
		t.Errorf("tooling-discord-community-host came from %q, want %q",
			admitted["tooling-discord-community-host"], second)
	}
}

// A missing catalogue is an error. Treating it as empty would turn a bad path
// into a bundle that is merely bare, which is the quiet failure to avoid.
func TestExpansionRefusesAnUnreadableCatalogue(t *testing.T) {
	t.Parallel()
	absent := filepath.Join(t.TempDir(), "not-a-checkout")
	if _, err := ExpandRole([]string{absent}, "creator", RoleGraph{}); err == nil {
		t.Fatal("a catalogue that does not exist was accepted")
	}
}

// Only directories are skills. A stray file must not become an admitted name.
func TestExpansionIgnoresFilesInTheCatalogue(t *testing.T) {
	t.Parallel()
	catalog := fixtureCatalog(t, "writing-house-style")
	stray := filepath.Join(catalog, ".agents", "composed", "writing-README.md")
	if err := os.WriteFile(stray, []byte("not a skill"), 0o644); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}
	admitted, _, err := ExpandRoleWithExclusions([]string{catalog}, "creator",
		RoleGraph{Patterns: map[string][]string{"creator": {"writing-*"}}})
	if err != nil {
		t.Fatalf("expansion errored: %v", err)
	}
	if names := SortedNames(admitted); len(names) != 1 || names[0] != "writing-house-style" {
		t.Errorf("admitted %v, want only the directory", names)
	}
}

// The declaration is generated and then diffed, so a set that renders in map
// order would churn on every build.
func TestRenderDeclarationIsDeterministicAndPathsTheSkills(t *testing.T) {
	t.Parallel()
	admitted := map[string]string{"writing-b": "x", "writing-a": "x", "writing-c": "x"}
	names := SortedNames(admitted)
	if strings.Join(names, ",") != "writing-a,writing-b,writing-c" {
		t.Fatalf("SortedNames = %v, want sorted", names)
	}
	rendered := RenderDeclaration("aos-public", names)
	if rendered != RenderDeclaration("aos-public", SortedNames(admitted)) {
		t.Error("two renders of one set disagree")
	}
	for _, expected := range []string{
		`source "aos-public" {`,
		`skill "writing-a" path="skills/writing-a"`,
		"Do not edit.",
	} {
		if !strings.Contains(rendered, expected) {
			t.Errorf("the declaration is missing %q", expected)
		}
	}
	if first, last := strings.Index(rendered, "writing-a"), strings.Index(rendered, "writing-c"); first > last {
		t.Error("the declaration did not render in sorted order")
	}
}
