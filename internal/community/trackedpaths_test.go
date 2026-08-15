package community

import (
	"path/filepath"
	"sort"
	"testing"
)

// Packs live in two places since sirens-echo#816: shared fixtures under agent/
// and each agent's canonical set under agents/<name>/packs/.
func globAll(t *testing.T, patterns ...[]string) []string {
	t.Helper()
	paths := make([]string, 0)
	for _, pattern := range patterns {
		found, err := filepath.Glob(filepath.Join(pattern...))
		if err != nil {
			t.Fatalf("glob %v: %v", pattern, err)
		}
		paths = append(paths, found...)
	}
	if len(paths) == 0 {
		t.Fatalf("no paths matched %v", patterns)
	}
	sort.Strings(paths)
	return paths
}

// trackedPackPaths is every pack the repository ships. Probes are preserved
// evidence rather than packs, so they stay out.
func trackedPackPaths(t *testing.T) []string {
	t.Helper()
	return globAll(t,
		[]string{"..", "..", "agent", "*.yaml"},
		[]string{"..", "..", "agents", "*", "packs", "*.yaml"},
	)
}

// trackedDefinitionPaths is one definition per agent.
func trackedDefinitionPaths(t *testing.T) []string {
	t.Helper()
	return globAll(t, []string{"..", "..", "agents", "*", "definition.yaml"})
}
