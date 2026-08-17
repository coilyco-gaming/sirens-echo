package community

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// The pointers were decorative: `Read references/x.md` instructed the model to
// read something already above it in the same prompt. See sirens-echo#859.

// skillRoot writes one root with an inline reference and an optional one.
func skillRoot(t *testing.T) string {
	t.Helper()
	root := t.TempDir()
	skill := filepath.Join(root, "demo")
	if err := os.MkdirAll(filepath.Join(skill, "references"), 0o755); err != nil {
		t.Fatalf("prepare root: %v", err)
	}
	write := func(name, body string) {
		if err := os.WriteFile(filepath.Join(skill, name), []byte(body), 0o600); err != nil {
			t.Fatalf("write %s: %v", name, err)
		}
	}
	write("SKILL.md", "# Demo\n\nThe index.\n")
	write(filepath.Join("references", "optional.md"), "# Optional\n\nThe long tail.\n")
	write(
		filepath.Join("references", "bounds.md"),
		"---\ninline: always\n---\n\n# Bounds\n\nWhat is declined.\n",
	)
	return skill
}

// A reference that must not be optional stays in every prompt, and the rest
// leave it. That split is the whole design.
func TestAnAlwaysInlineReferenceStaysInThePrompt(t *testing.T) {
	t.Parallel()
	root := skillRoot(t)

	pack, err := LoadSkillpack([]string{root})
	if err != nil {
		t.Fatalf("LoadSkillpack: %v", err)
	}
	if !strings.Contains(pack, "What is declined.") {
		t.Errorf("a reference marked inline left the prompt:\n%s", pack)
	}
	if strings.Contains(pack, "The long tail.") {
		t.Errorf("an optional reference was inlined:\n%s", pack)
	}
	// A file the model cannot see is a file it will not ask for.
	if !strings.Contains(pack, "references/optional.md") {
		t.Errorf("the pack does not index what is readable:\n%s", pack)
	}
	if !strings.Contains(pack, "Optional") {
		t.Errorf("the index does not say what the reference is about:\n%s", pack)
	}
}

func skillTool(t *testing.T) ToolSession {
	t.Helper()
	references, err := LoadSkillReferences([]string{skillRoot(t)})
	if err != nil {
		t.Fatalf("LoadSkillReferences: %v", err)
	}
	session, err := (&SkillProvider{References: references}).Open(context.Background())
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	return session
}

// The pointer is true now: asking for the path returns the file.
func TestReadingASkillReferenceReturnsIt(t *testing.T) {
	t.Parallel()
	session := skillTool(t)
	tools := session.Tools()
	if len(tools) != 1 || tools[0].Name != skillToolName {
		t.Fatalf("tools = %#v", tools)
	}

	path := session.(*skillSession).paths()[0]
	result, err := session.Call(
		context.Background(), skillToolName, map[string]any{"path": path})
	if err != nil {
		t.Fatalf("Call: %v", err)
	}
	if result.IsError || !strings.Contains(result.Text, "The long tail.") {
		t.Errorf("read %q returned %#v", path, result)
	}
}

// A guessed path refuses with what is available, or a model that guessed once
// guesses again.
func TestAnUnknownSkillPathRefusesWithTheList(t *testing.T) {
	t.Parallel()
	session := skillTool(t)

	result, err := session.Call(
		context.Background(), skillToolName, map[string]any{"path": "references/nope.md"})
	if err != nil {
		t.Fatalf("Call: %v", err)
	}
	if !result.IsError {
		t.Fatal("an unknown path was served")
	}
	if !strings.Contains(result.Text, "optional.md") {
		t.Errorf("the refusal does not say what is readable: %q", result.Text)
	}
}

// An inline reference is not fetchable, because it is already in the prompt and
// serving it twice would pay the cost this change removes.
func TestAnInlineReferenceIsNotAlsoFetchable(t *testing.T) {
	t.Parallel()
	references, err := LoadSkillReferences([]string{skillRoot(t)})
	if err != nil {
		t.Fatalf("LoadSkillReferences: %v", err)
	}
	for _, reference := range references {
		if strings.Contains(reference.Body, "What is declined.") {
			t.Errorf("an inline reference is also in the catalog: %s", reference.Path)
		}
	}
}

// A root with no optional references offers no tool at all, rather than a tool
// that refuses everything.
func TestARootWithNothingFetchableOffersNoTool(t *testing.T) {
	t.Parallel()
	session, err := (&SkillProvider{}).Open(context.Background())
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	if len(session.Tools()) != 0 {
		t.Errorf("tools = %#v", session.Tools())
	}
	if _, err := session.Call(
		context.Background(), skillToolName, map[string]any{"path": "x"},
	); err == nil {
		t.Error("a tool with nothing to serve accepted a call")
	}
}
