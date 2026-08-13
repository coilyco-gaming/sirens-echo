package main_test

import (
	"os"
	"strings"
	"testing"
)

// The gate verb exists so one habit replaces six. A check CI runs and the gate
// omits is a check a push can miss while the gate reports ready. See issue 305.
func TestTheGateCoversEveryCheckCIRuns(t *testing.T) {
	t.Parallel()
	gate, err := os.ReadFile("../../scripts/ward-command.sh")
	if err != nil {
		t.Fatalf("read the ward command script: %v", err)
	}
	workflow, err := os.ReadFile("../../.forgejo/workflows/ci.yml")
	if err != nil {
		t.Fatalf("read the CI workflow: %v", err)
	}
	body := string(gate)
	for _, verb := range []string{"build", "policy-check", "vet", "test", "test-skips"} {
		if !strings.Contains(body, verb) {
			t.Errorf("the gate omits %q, which CI runs", verb)
		}
		if !strings.Contains(string(workflow), "ward exec "+verb) {
			t.Errorf("CI no longer runs %q; drop it from the gate or restore it", verb)
		}
	}
	if !strings.Contains(body, "pre-commit run --all-files") {
		t.Error("the gate does not run pre-commit, which is the step that keeps failing")
	}
}
