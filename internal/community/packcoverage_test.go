package community

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// policy-check names every pack it verifies by hand, so a new pack is verified
// by nothing until someone remembers. That is a quiet success, not a failure.

// unverifiedPacks exempts a tracked file that policy-check deliberately skips.
// An entry is a reviewed line rather than a number, and there are none today.
var unverifiedPacks = map[string]string{}

func TestEveryTrackedPackReachesPolicyCheck(t *testing.T) {
	t.Parallel()
	paths, err := filepath.Glob(filepath.Join("..", "..", "agent", "*.yaml"))
	if err != nil || len(paths) == 0 {
		t.Fatalf("glob agent packs: %v, found %d", err, len(paths))
	}
	source, err := os.ReadFile(filepath.Join(
		"..", "..", "cmd", "sirens-echo-policy-check", "main.go",
	))
	if err != nil {
		t.Fatalf("read policy-check: %v", err)
	}
	body := string(source)

	for _, path := range paths {
		relative := "agent/" + filepath.Base(path)
		reason, exempt := unverifiedPacks[relative]
		verified := strings.Contains(body, `"`+relative+`"`)
		switch {
		case verified && exempt:
			t.Errorf("%s is verified now, so drop it from unverifiedPacks and let "+
				"this assert the invariant. It was exempt because: %s", relative, reason)
		case !verified && !exempt:
			t.Errorf("%s is tracked and policy-check never loads it, so nothing "+
				"validates its schema or contents. Add it there", relative)
		}
	}
}

// A pack that declares no schema would load as whichever type a caller guessed,
// so the schema line is what makes the file self-describing.
func TestEveryTrackedPackDeclaresItsSchema(t *testing.T) {
	t.Parallel()
	paths, err := filepath.Glob(filepath.Join("..", "..", "agent", "*.yaml"))
	if err != nil || len(paths) == 0 {
		t.Fatalf("glob agent packs: %v, found %d", err, len(paths))
	}
	for _, path := range paths {
		body, err := os.ReadFile(path)
		if err != nil {
			t.Fatalf("read %s: %v", path, err)
		}
		declared := false
		for _, line := range strings.Split(string(body), "\n") {
			if strings.HasPrefix(strings.TrimSpace(line), "schema:") {
				declared = true
				break
			}
		}
		if !declared {
			t.Errorf("%s declares no schema, so a loader cannot refuse the wrong file",
				filepath.Base(path))
		}
	}
}
