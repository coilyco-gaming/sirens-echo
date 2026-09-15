package main

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// definitionAt writes a definition whose only variable is its skill roots,
// which is the field this tool exists for.
func definitionAt(t *testing.T, roots ...string) string {
	t.Helper()
	var body strings.Builder
	body.WriteString("schema: coilyco-harness.agent.v1\n")
	body.WriteString("identity: Sirens Echo\n")
	body.WriteString("audit_role: community\n")
	body.WriteString("response_style: neutral\n")
	body.WriteString("max_context_messages: 12\n")
	body.WriteString("local_skill_roots:\n")
	for _, root := range roots {
		body.WriteString("  - " + root + "\n")
	}
	path := filepath.Join(t.TempDir(), "definition.yaml")
	if err := os.WriteFile(path, []byte(body.String()), 0o600); err != nil {
		t.Fatalf("write definition: %v", err)
	}
	return path
}

// skillRootAt builds the smallest tree the skillpack loader accepts.
func skillRootAt(t *testing.T) string {
	t.Helper()
	root := filepath.Join(t.TempDir(), "a-skill")
	if err := os.MkdirAll(root, 0o755); err != nil {
		t.Fatalf("create skill root: %v", err)
	}
	body := "---\nname: a-skill\ndescription: one skill, for a test.\n---\n\nA line of policy.\n"
	if err := os.WriteFile(filepath.Join(root, "SKILL.md"), []byte(body), 0o600); err != nil {
		t.Fatalf("write SKILL.md: %v", err)
	}
	return root
}

// The outage this exists for: deploy#666 named a root the image lacked and the
// lane crashlooped for 90 minutes with no signal at edit time. sirens-echo#973.
func TestARootTheTreeDoesNotCarryFails(t *testing.T) {
	t.Parallel()
	path := definitionAt(t, filepath.Join(t.TempDir(), "sirens-fixture"))

	var stdout, stderr bytes.Buffer
	if code := run([]string{path}, &stdout, &stderr); code != 1 {
		t.Errorf("exit = %d, want 1", code)
	}
	if !strings.Contains(stderr.String(), "walk skill root") {
		t.Errorf("stderr = %q, want the walk failure the pod reports", stderr.String())
	}
	if !strings.Contains(stderr.String(), "sirens-fixture") {
		t.Errorf("stderr = %q, want the missing root named", stderr.String())
	}
}

// A definition whose roots are all present has to pass, or a gate nobody can
// get green is worse than no gate.
func TestARootTheTreeCarriesPasses(t *testing.T) {
	t.Parallel()
	path := definitionAt(t, skillRootAt(t))

	var stdout, stderr bytes.Buffer
	if code := run([]string{path}, &stdout, &stderr); code != 0 {
		t.Fatalf("exit = %d, stderr = %q", code, stderr.String())
	}
	if !strings.Contains(stdout.String(), "ok") {
		t.Errorf("stdout = %q, want an ok line", stdout.String())
	}
	// The roots it resolved, not only that it resolved some. A root can exist
	// and still be the wrong one.
	if !strings.Contains(stdout.String(), "local skill roots (1)") {
		t.Errorf("stdout = %q, want the roots reported", stdout.String())
	}
}

// deploy pipes a ConfigMap key, so the argument it uses has to work.
func TestTheStdinArgumentIsAccepted(t *testing.T) {
	path := definitionAt(t, skillRootAt(t))
	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read definition: %v", err)
	}
	piped := filepath.Join(t.TempDir(), "piped.yaml")
	if err := os.WriteFile(piped, raw, 0o600); err != nil {
		t.Fatalf("write piped: %v", err)
	}
	file, err := os.Open(piped)
	if err != nil {
		t.Fatalf("open piped: %v", err)
	}
	t.Cleanup(func() { _ = file.Close() })
	// Not parallel: this swaps the process's stdin.
	original := os.Stdin
	os.Stdin = file
	t.Cleanup(func() { os.Stdin = original })

	var stdout, stderr bytes.Buffer
	if code := run([]string{"-"}, &stdout, &stderr); code != 0 {
		t.Fatalf("exit = %d, stderr = %q", code, stderr.String())
	}
}

// No argument is a caller mistake rather than a bad definition, and deploy's CI
// keys on the difference.
func TestNoArgumentIsItsOwnExitCode(t *testing.T) {
	t.Parallel()
	var stdout, stderr bytes.Buffer
	if code := run(nil, &stdout, &stderr); code != 2 {
		t.Errorf("exit = %d, want 2", code)
	}
	if !strings.Contains(stderr.String(), "usage:") {
		t.Errorf("stderr = %q, want the usage line", stderr.String())
	}
}
