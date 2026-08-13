package community

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// Opting a run into a real bundle closes the instruction gap issue 316 measured.
// The stub stays the default, so an existing run does not change meaning.

func writeBundle(t *testing.T, card string) string {
	t.Helper()
	dir := t.TempDir()
	content := filepath.Join(dir, "content")
	if err := os.MkdirAll(content, 0o755); err != nil {
		t.Fatalf("make bundle dir: %v", err)
	}
	path := filepath.Join(content, "instructions.md")
	if err := os.WriteFile(path, []byte(card), 0o644); err != nil {
		t.Fatalf("write identity card: %v", err)
	}
	// LoadBundle reads the identity card and every selected skill root.
	skill := filepath.Join(content, "skills", "role", "role-engineer")
	if err := os.MkdirAll(skill, 0o755); err != nil {
		t.Fatalf("make skill dir: %v", err)
	}
	if err := os.WriteFile(filepath.Join(skill, "SKILL.md"), []byte("# Engineer\n\nBuild it."), 0o644); err != nil {
		t.Fatalf("write skill: %v", err)
	}
	return dir
}

func TestComposedForRunStaysStubbedWithoutTheEnv(t *testing.T) {
	t.Setenv(ComposedBundleEnv, "")
	bundle, recorded, err := composedForRun(Definition{Composed: true})
	if err != nil {
		t.Fatalf("composedForRun: %v", err)
	}
	if bundle != PlaceholderComposed || recorded != ComposedStubbed {
		t.Fatalf("the default stopped being the stub: recorded %q", recorded)
	}
}

func TestComposedForRunReadsARealBundleWhenAsked(t *testing.T) {
	dir := writeBundle(t, "# Role instructions\n\nEngineer, with a real bundle.")
	t.Setenv(ComposedBundleEnv, dir)
	bundle, recorded, err := composedForRun(Definition{Composed: true})
	if err != nil {
		t.Fatalf("composedForRun: %v", err)
	}
	if bundle == PlaceholderComposed {
		t.Fatal("the run read the stub while a bundle was supplied")
	}
	if !strings.Contains(bundle, "Engineer, with a real bundle") {
		t.Errorf("bundle did not carry the identity card: %q", bundle)
	}
	// The label has to name what was read, or a dataset cannot be told apart
	// from a stubbed one. See issue 316.
	if !strings.Contains(recorded, dir) {
		t.Errorf("recorded %q does not name the bundle", recorded)
	}
	if recorded == ComposedStubbed {
		t.Error("a real bundle recorded itself as stubbed")
	}
}

// A silent fallback would produce a dataset labelled with a bundle the run
// never read, which is worse than the gap this feature closes.
func TestComposedForRunFailsRatherThanFallingBack(t *testing.T) {
	t.Setenv(ComposedBundleEnv, filepath.Join(t.TempDir(), "does-not-exist"))
	bundle, recorded, err := composedForRun(Definition{Composed: true})
	if err == nil {
		t.Fatalf("an unreadable bundle was tolerated: bundle=%q recorded=%q", bundle, recorded)
	}
	if bundle != "" || recorded != "" {
		t.Error("a failed load still returned a bundle or a label")
	}
	if !strings.Contains(err.Error(), ComposedBundleEnv) {
		t.Errorf("error does not name the variable that caused it: %v", err)
	}
}

// An uncomposed definition ignores the variable entirely: Echo is not composed
// and must not silently gain a bundle from an operator's environment.
func TestComposedForRunIgnoresTheEnvWhenNotComposed(t *testing.T) {
	t.Setenv(ComposedBundleEnv, writeBundle(t, "# Role instructions\n\nShould not be read."))
	bundle, recorded, err := composedForRun(Definition{Composed: false})
	if err != nil {
		t.Fatalf("composedForRun: %v", err)
	}
	if bundle != "" || recorded != ComposedNotRequested {
		t.Fatalf("an uncomposed definition read a bundle: recorded %q", recorded)
	}
}
