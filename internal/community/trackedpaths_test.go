package community

import (
	"path/filepath"
	"sort"
	"strings"
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

// trackedRatePackPaths is each agent's rate pack, the only packs carrying the
// shape a comparison reads.
func trackedRatePackPaths(t *testing.T) []string {
	t.Helper()
	return globAll(t, []string{"..", "..", "agents", "*", "packs", "rate.yaml"})
}

// agentOf reads the segment after `agents/`, not directories up from the file:
// a definition sits shallower than a pack, and counting up returned "agents".
func agentOf(path string) string {
	parts := strings.Split(filepath.ToSlash(path), "/")
	for i, part := range parts {
		if part == "agents" && i+1 < len(parts) {
			return parts[i+1]
		}
	}
	return filepath.Base(filepath.Dir(path))
}

// definitionOf is one named agent's definition, for a test that is about that
// agent rather than about a property every agent shares.
func definitionOf(t *testing.T, agent string) string {
	t.Helper()
	for _, path := range trackedDefinitionPaths(t) {
		if agentOf(path) == agent {
			return path
		}
	}
	t.Fatalf("no tracked definition for agent %q", agent)
	return ""
}
