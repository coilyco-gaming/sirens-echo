package community

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// The fixture is written at test time rather than tracked, so this stays
// hermetic and no bundle markdown lands in the tree. See #125.
const fixtureInstructions = `# Role instructions

Agent-compose assigned the ` + "`fixture`" + ` role from the caller's compose request.

**Role skill // ` + "`role-fixture`" + `**
**Agent // Fixture (they)**

## Personality meld

Fixture personalities, for the test only.
`

const fixtureManifest = `{
  "format": "agent-compose.bundle",
  "role": "fixture",
  "role_skill": "role-fixture",
  "model_tier": "frontier",
  "personalities": ["warm", "bold"],
  "sources": ["roster:core", "aos-public"],
  "content": [{"id": "roster:core:role:fixture"}]
}`

func composedFixtureDefinition() Definition {
	return Definition{
		Schema:             "coilyco-harness.agent.v1",
		Identity:           "Sirens Deep of Coilyco",
		AuditRole:          "general",
		ResponseStyle:      ResponseStyleSocial,
		Composed:           true,
		MaxContextMessages: 12,
		LocalSkillRoots:    []string{".agents/skills/coilyco-general"},
	}
}

// writeRoleBundle lays out one baked role the way agent-compose does, down
// to the escaped source directory name.
func writeRoleBundle(t *testing.T, root, role, body string) string {
	t.Helper()
	dir := filepath.Join(root, role)
	skills := map[string]string{
		// `roster:core` reaches the filesystem escaped.
		filepath.Join("roster%3Acore", "personality-warm"):      "# Warm\n",
		filepath.Join("aos-public", "writing-voice-adaptation"): body,
	}
	for path, content := range skills {
		full := filepath.Join(dir, "content", "skills", path)
		if err := os.MkdirAll(full, 0o755); err != nil {
			t.Fatalf("prepare %s: %v", path, err)
		}
		if err := os.WriteFile(filepath.Join(full, "SKILL.md"), []byte(content), 0o644); err != nil {
			t.Fatalf("write %s: %v", path, err)
		}
	}
	instructions := filepath.Join(dir, "content", "instructions.md")
	if err := os.WriteFile(instructions, []byte(fixtureInstructions), 0o644); err != nil {
		t.Fatalf("write instructions: %v", err)
	}
	if err := os.WriteFile(filepath.Join(dir, "manifest.json"), []byte(fixtureManifest), 0o644); err != nil {
		t.Fatalf("write manifest: %v", err)
	}
	return dir
}

func loadFixture(t *testing.T, root string) []RoleBundle {
	t.Helper()
	loaded, err := LoadRoleBundles(
		root,
		composedFixtureDefinition(),
		PlaceholderPrincipal,
		"local policy",
	)
	if err != nil {
		t.Fatalf("LoadRoleBundles: %v", err)
	}
	return loaded
}

func TestRoleSnapshotRecordsTheSelection(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	writeRoleBundle(t, root, "fixture", "# Voice adaptation\n")
	loaded := loadFixture(t, root)
	if len(loaded) != 1 || loaded[0].Role != "fixture" {
		t.Fatalf("loaded = %#v", loaded)
	}
	snapshot := RenderRoleSnapshot(loaded[0])
	for _, expected := range []string{
		"Role: fixture",
		"Role skill: role-fixture",
		"Personalities: bold, warm",
		"Sources: aos-public, roster:core",
		"Skills: 2",
		"aos-public/writing-voice-adaptation",
		// The escaped directory decodes back to the source's real name.
		"roster:core/personality-warm",
	} {
		if !strings.Contains(snapshot, expected) {
			t.Errorf("snapshot missing %q:\n%s", expected, snapshot)
		}
	}
}

// The catalogue ref floats, so a body edit upstream must not move the record.
// Otherwise the gate reddens main for a change nobody here can review.
func TestRoleSnapshotIgnoresAnUpstreamBodyEdit(t *testing.T) {
	t.Parallel()
	before := t.TempDir()
	writeRoleBundle(t, before, "fixture", "# Voice adaptation\n")
	after := t.TempDir()
	writeRoleBundle(t, after, "fixture", "# Rewritten upstream\n\nMany more words.\n")

	original := RenderRoleSnapshot(loadFixture(t, before)[0])
	edited := RenderRoleSnapshot(loadFixture(t, after)[0])

	if original != edited {
		t.Errorf("an upstream body edit moved the record:\n%s\n---\n%s", original, edited)
	}
}

// A selection change is the drift worth reviewing, so it has to move it.
func TestRoleSnapshotMovesWhenASkillIsAdded(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	dir := writeRoleBundle(t, root, "fixture", "# Voice adaptation\n")
	original := RenderRoleSnapshot(loadFixture(t, root)[0])

	added := filepath.Join(dir, "content", "skills", "aos-public", "writing-kai-voice")
	if err := os.MkdirAll(added, 0o755); err != nil {
		t.Fatalf("add a skill: %v", err)
	}
	if err := os.WriteFile(filepath.Join(added, "SKILL.md"), []byte("# Voice\n"), 0o644); err != nil {
		t.Fatalf("write the added skill: %v", err)
	}
	widened := RenderRoleSnapshot(loadFixture(t, root)[0])

	if widened == original {
		t.Error("a role gaining a skill did not move its record")
	}
	if !strings.Contains(widened, "Skills: 3") {
		t.Errorf("record did not count the added skill:\n%s", widened)
	}
}

// A bundle that failed to compose must stop the build rather than ship as a
// quietly neutral agent, which is what #98 asked the gate to prevent.
func TestLoadRoleBundlesRejectsABundleWithNoComposedSurface(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	role := filepath.Join(root, "hollow")
	if err := os.MkdirAll(filepath.Join(role, "content", "skills"), 0o755); err != nil {
		t.Fatalf("prepare bundle: %v", err)
	}
	manifest := `{"format":"agent-compose.bundle","role":"hollow","sources":[]}`
	if err := os.WriteFile(filepath.Join(role, "manifest.json"), []byte(manifest), 0o644); err != nil {
		t.Fatalf("write manifest: %v", err)
	}
	instructions := filepath.Join(role, "content", "instructions.md")
	if err := os.WriteFile(instructions, []byte("nothing composed here\n"), 0o644); err != nil {
		t.Fatalf("write instructions: %v", err)
	}
	_, err := LoadRoleBundles(root, composedFixtureDefinition(), PlaceholderPrincipal, "local policy")
	if err == nil {
		t.Fatal("accepted a bundle carrying no composed surface")
	}
	if !strings.Contains(err.Error(), "hollow") {
		t.Errorf("error does not name the role: %v", err)
	}
}

// A misfiled bundle would make SIRENS_ECHO_ROLE select the wrong identity, and
// nothing downstream would notice.
func TestLoadRoleBundlesRejectsAMisfiledRole(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	role := filepath.Join(root, "director")
	if err := os.MkdirAll(filepath.Join(role, "content", "skills"), 0o755); err != nil {
		t.Fatalf("prepare bundle: %v", err)
	}
	manifest := `{"format":"agent-compose.bundle","role":"creator","sources":[]}`
	if err := os.WriteFile(filepath.Join(role, "manifest.json"), []byte(manifest), 0o644); err != nil {
		t.Fatalf("write manifest: %v", err)
	}
	_, err := LoadRoleBundles(root, composedFixtureDefinition(), PlaceholderPrincipal, "local policy")
	if err == nil {
		t.Fatal("accepted a bundle filed under the wrong role slug")
	}
}

func TestLoadRoleBundlesRejectsAnEmptyBundleDirectory(t *testing.T) {
	t.Parallel()
	if _, err := LoadRoleBundles(t.TempDir(), composedFixtureDefinition(), PlaceholderPrincipal, "p"); err == nil {
		t.Fatal("accepted a bundle directory with no role in it")
	}
}
