package community

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestLoadSkillpackLoadsSkillAndReferences(t *testing.T) {
	t.Parallel()
	communityRoot := filepath.Join("..", "..", ".agents", "skills", "sirens-echo-community")
	knowledgeRoot := filepath.Join("..", "..", ".agents", "skills", "sirens-echo-knowledge")
	pack, err := LoadSkillpack([]string{communityRoot, knowledgeRoot})
	if err != nil {
		t.Fatalf("LoadSkillpack: %v", err)
	}
	for _, expected := range []string{
		"Sirens Echo response policy",
		"Keep responses neutral",
		"Sirens community knowledge",
		"#bots",
	} {
		if !strings.Contains(pack, expected) {
			t.Fatalf("skillpack missing %q", expected)
		}
	}
	if strings.Contains(pack, "name: sirens-echo-community") {
		t.Fatal("skillpack leaked YAML frontmatter")
	}
	if strings.Contains(pack, "openai.yaml") {
		t.Fatal("skillpack loaded UI metadata")
	}
}

func TestLoadSkillpackLoadsCoilyCoPolicySeparately(t *testing.T) {
	t.Parallel()
	generalRoot := filepath.Join("..", "..", ".agents", "skills", "coilyco-general")
	pack, err := LoadSkillpack([]string{generalRoot})
	if err != nil {
		t.Fatalf("LoadSkillpack: %v", err)
	}
	for _, expected := range []string{
		"CoilyCo general-purpose response policy",
		"Start from the request",
		"General-purpose means topic-neutral and extensible",
	} {
		if !strings.Contains(pack, expected) {
			t.Fatalf("general skillpack missing %q", expected)
		}
	}
	for _, forbidden := range []string{"Sirens community knowledge", "current Eco"} {
		if strings.Contains(pack, forbidden) {
			t.Fatalf("general skillpack retained %q", forbidden)
		}
	}
}

// A composed agent-compose source ships COMPOSED.md rather than SKILL.md, and
// a role bundle is unreadable without it.
func TestLoadSkillpackReadsComposedSources(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	composed := filepath.Join(root, "coilyco-favorites")
	if err := os.MkdirAll(filepath.Join(composed, "references"), 0o755); err != nil {
		t.Fatalf("prepare composed root: %v", err)
	}
	write := func(path, body string) {
		if err := os.WriteFile(path, []byte(body), 0o600); err != nil {
			t.Fatalf("write %s: %v", path, err)
		}
	}
	write(
		filepath.Join(composed, "COMPOSED.md"),
		"---\nname: coilyco-favorites\n---\n\n# Favorites\n\nPurple and black.\n",
	)
	write(filepath.Join(composed, "references", "detail.md"), "Reference detail.\n")

	ordinary := filepath.Join(root, "ordinary")
	if err := os.MkdirAll(ordinary, 0o755); err != nil {
		t.Fatalf("prepare ordinary root: %v", err)
	}
	write(filepath.Join(ordinary, "SKILL.md"), "# Ordinary\n\nOrdinary body.\n")

	pack, err := LoadSkillpack([]string{composed, ordinary})
	if err != nil {
		t.Fatalf("LoadSkillpack: %v", err)
	}
	for _, expected := range []string{"Purple and black.", "Reference detail.", "Ordinary body."} {
		if !strings.Contains(pack, expected) {
			t.Fatalf("skillpack missing %q:\n%s", expected, pack)
		}
	}
	if strings.Contains(pack, "name: coilyco-favorites") {
		t.Fatal("composed frontmatter reached the model context")
	}
}
