package community

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// Identity doctrine is not fetchable: an agent that has to ask what its own
// boundaries say has already acted without them.

func writeTwoSourceBundle(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	write := func(path, body string) {
		t.Helper()
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			t.Fatalf("make %s: %v", filepath.Dir(path), err)
		}
		if err := os.WriteFile(path, []byte(body), 0o644); err != nil {
			t.Fatalf("write %s: %v", path, err)
		}
	}
	write(filepath.Join(dir, "content", "instructions.md"), PlaceholderComposed)
	// The escaped directory name is what agent-compose actually stages.
	write(filepath.Join(dir, "content", "skills", "roster%3Acore", "boundary-modify-live-system", "SKILL.md"),
		"---\nname: boundary-modify-live-system\ndescription: Defer live-system changes.\n---\n\n# Boundary\n\nDevOps owns the mutation.")
	write(filepath.Join(dir, "content", "skills", "aos-public", "coding-go", "SKILL.md"),
		"---\nname: coding-go\ndescription: Go conventions.\n---\n\n# Go\n\nUse urfave/cli.")
	return dir
}

func TestRosterCoreLoadsEagerly(t *testing.T) {
	pack, err := LoadBundle(writeTwoSourceBundle(t))
	if err != nil {
		t.Fatalf("load bundle: %v", err)
	}
	if !strings.Contains(pack, "DevOps owns the mutation.") {
		t.Fatalf("roster:core body missing from the pack:\n%s", pack)
	}
	if strings.Contains(pack, "Use urfave/cli.") {
		t.Fatalf("an ordinary catalogue skill was inlined:\n%s", pack)
	}
	if !strings.Contains(pack, "Go conventions.") {
		t.Fatalf("the deferred skill lost its description:\n%s", pack)
	}
}

// An eager root is fully present, so nothing about it is left to fetch.
func TestRosterCoreOffersNothingToRead(t *testing.T) {
	references, err := LoadBundleReferences(writeTwoSourceBundle(t))
	if err != nil {
		t.Fatalf("load references: %v", err)
	}
	for _, reference := range references {
		if strings.Contains(reference.Path, "roster") {
			t.Fatalf("roster:core was offered as fetchable: %s", reference.Path)
		}
	}
	if len(references) != 1 {
		t.Fatalf("want the one catalogue skill fetchable, got %d", len(references))
	}
}
