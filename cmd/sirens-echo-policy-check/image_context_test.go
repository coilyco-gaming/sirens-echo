package main_test

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

// imageContextPaths mirrors the Dockerfile's COPY set for the stage that runs
// this binary. Anything outside it is absent when the image builds.
var imageContextPaths = []string{
	"agent",
	"agents",
	filepath.Join(".agents", "skills"),
	"docs",
}

// The publish job that catches a missing path runs only after a merge to main,
// so this test is that gate moved earlier. See docs/sirens-echo-access.md.
func TestPolicyCheckRunsInsideTheImageContext(t *testing.T) {
	if _, err := exec.LookPath("go"); err != nil {
		t.Skip("go toolchain unavailable")
	}
	repoRoot, err := filepath.Abs(filepath.Join("..", ".."))
	if err != nil {
		t.Fatalf("resolve repo root: %v", err)
	}

	workDir := t.TempDir()
	binary := filepath.Join(workDir, "policy-check")
	build := exec.Command("go", "build", "-o", binary, "./cmd/sirens-echo-policy-check")
	build.Dir = repoRoot
	if output, err := build.CombinedOutput(); err != nil {
		t.Fatalf("build policy-check: %v\n%s", err, output)
	}

	context := filepath.Join(workDir, "context")
	for _, relative := range imageContextPaths {
		source := filepath.Join(repoRoot, relative)
		if _, err := os.Stat(source); err != nil {
			t.Fatalf("Dockerfile copies %s but it is missing: %v", relative, err)
		}
		target := filepath.Join(context, relative)
		if err := os.MkdirAll(filepath.Dir(target), 0o755); err != nil {
			t.Fatalf("prepare %s: %v", relative, err)
		}
		if output, err := exec.Command("cp", "-R", source, target).CombinedOutput(); err != nil {
			t.Fatalf("copy %s: %v\n%s", relative, err, output)
		}
	}

	run := exec.Command(binary)
	run.Dir = context
	// The deployment's own policy path is unset during an image build.
	run.Env = append(os.Environ(), "SIRENS_ECHO_ACCESS_POLICY=")
	output, err := run.CombinedOutput()
	if err != nil {
		t.Fatalf(
			"policy-check failed with only the image's files, so the image build will fail: %v\n%s",
			err,
			output,
		)
	}
	if !strings.Contains(string(output), "verified") {
		t.Fatalf("policy-check verified nothing: %s", output)
	}
}
