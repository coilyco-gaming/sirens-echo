package community

import (
	"os"
	"path/filepath"
	"sort"
	"strings"
	"testing"
)

// reactionRefused existed and fired on nothing, so the state most specifically
// about a content boundary was silent on the only one. See sirens-echo#447.

// reactionNames are the four states, checked against the file that declares
// them so a fifth cannot be added without meeting this test.
var reactionNames = []string{
	"reactionAccepted",
	"reactionTool",
	"reactionFailed",
	"reactionRefused",
}

// A reaction nobody emits is a state the member never sees, which is the defect
// 447 found by reading rather than by any test failing.
func TestEveryReactionIsEmittedSomewhere(t *testing.T) {
	t.Parallel()
	code := nonTestGoSources(t)
	silent := make([]string, 0)
	for _, name := range reactionNames {
		emitted := false
		for path, body := range code {
			if filepath.Base(path) == "reactions.go" {
				continue
			}
			if strings.Contains(body, name) {
				emitted = true
				break
			}
		}
		if !emitted {
			silent = append(silent, name)
		}
	}
	sort.Strings(silent)
	if len(silent) > 0 {
		t.Errorf("%s is declared and never emitted, so the state it names cannot "+
			"reach a member. Wire it or delete it", strings.Join(silent, ", "))
	}
}

// The list above is hand-kept, so a constant added to reactions.go without an
// entry would be silently exempt from the check that matters.
func TestEveryDeclaredReactionIsListedHere(t *testing.T) {
	t.Parallel()
	body, err := os.ReadFile("reactions.go")
	if err != nil {
		t.Fatalf("read reactions.go: %v", err)
	}
	declared := make([]string, 0, len(reactionNames))
	for _, line := range strings.Split(string(body), "\n") {
		trimmed := strings.TrimSpace(line)
		name, _, found := strings.Cut(trimmed, " = ")
		if found && strings.HasPrefix(name, "reaction") {
			declared = append(declared, name)
		}
	}
	sort.Strings(declared)
	known := append([]string(nil), reactionNames...)
	sort.Strings(known)
	if strings.Join(declared, ",") != strings.Join(known, ",") {
		t.Errorf("reactions.go declares %v and this test knows %v. A reaction "+
			"missing from the list is exempt from the emission check", declared, known)
	}
}

// nonTestGoSources reads the package's own non-test files once.
func nonTestGoSources(t *testing.T) map[string]string {
	t.Helper()
	paths, err := filepath.Glob("*.go")
	if err != nil || len(paths) == 0 {
		t.Fatalf("glob package sources: %v, found %d", err, len(paths))
	}
	sources := make(map[string]string, len(paths))
	for _, path := range paths {
		if strings.HasSuffix(path, "_test.go") {
			continue
		}
		body, err := os.ReadFile(path)
		if err != nil {
			t.Fatalf("read %s: %v", path, err)
		}
		sources[path] = string(body)
	}
	return sources
}
