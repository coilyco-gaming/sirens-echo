package community

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
)

// The role graph is agent-compose's own format, so the allowlist stays terse
// and globbed. See docs/sirens-echo-compose.md.

// DeniedComposedSkills must never reach an agent that answers strangers. Every
// name must resolve, which is what VerifyDenyListResolves holds true.
var DeniedComposedSkills = map[string]string{
	"kai-career":                 "private career context",
	"kai-job-search":             "private job search",
	"kai-grill-me":               "private operating context",
	"tooling-collaboration":      "private collaboration context",
	"kapwing-pr-review":          "employer team and domain context",
	"kai-bio-surface":            "resume and identity surface, points at private lore",
	"writing-public-repos":       "names private sibling repositories",
	"coilyco-design-language":    "art direction, and low-context: required",
	"personal-preference-social": "an organization cannot own a person's social accounts",
	"tooling-cross-repo-infra":   "fleet mutation surface",
}

// RetiredDeniedSkills left the catalogues for voice-corpus. Still denied, so a
// revival under the old name is refused rather than admitted by an absence.
var RetiredDeniedSkills = map[string]string{
	"kai-linkedin-voice":    "a member's personal channel voice, moved to voice-corpus",
	"kai-linkedin-video":    "a member's personal channel format, moved to voice-corpus",
	"kai-engineering-voice": "code review and eng-channel posts, moved to voice-corpus",
}

// deniedReason reports why a name is refused, across both halves of the list.
func deniedReason(name string) (string, bool) {
	if reason, ok := DeniedComposedSkills[name]; ok {
		return reason, true
	}
	reason, ok := RetiredDeniedSkills[name]
	return reason, ok
}

// PrivateRepositories must never be globalized. A public repository is fine.
var PrivateRepositories = map[string]string{
	"coilysiren/lore":                    "private durable context",
	"coilysiren/inbox":                   "private work intake",
	"coilysiren/voice-corpus":            "private dataset",
	"coilyco-bridge/agentic-os-xxx":      "private harness",
	"coilyco-bridge/agentic-os-kai":      "private personal catalogue",
	"coilyco-flight-deck/infrastructure": "host and cluster operations",
	"coilyco-bridge/deploy":              "cluster deployment surface",
}

// RoleGraph is the parsed allowlist: per-role composed-skill patterns plus the
// repository declarations the graph globalizes.
type RoleGraph struct {
	Patterns     map[string][]string
	Repositories map[string]string
	Globals      []string
}

var (
	graphRole       = regexp.MustCompile(`^\s*role\s+"?([a-z][a-z0-9-]*)"?\s*\{`)
	graphSkill      = regexp.MustCompile(`^\s*composed-skill\s+"?([^"\s]+)"?\s*$`)
	graphRepository = regexp.MustCompile(`^\s*repository\s+(\S+)\s+path="([^"]+)"`)
	graphGlobal     = regexp.MustCompile(`^\s*global\s+(\S+)\s*$`)
)

// ParseRoleGraph reads the tracked allowlist. Line-oriented on purpose: the
// file is small and a KDL dependency would buy nothing.
func ParseRoleGraph(body string) RoleGraph {
	graph := RoleGraph{Patterns: map[string][]string{}, Repositories: map[string]string{}}
	role := ""
	for _, line := range strings.Split(body, "\n") {
		trimmed := strings.TrimSpace(line)
		if strings.HasPrefix(trimmed, "//") {
			continue
		}
		if match := graphRepository.FindStringSubmatch(line); match != nil {
			graph.Repositories[match[1]] = match[2]
			continue
		}
		if match := graphGlobal.FindStringSubmatch(line); match != nil {
			graph.Globals = append(graph.Globals, match[1])
			continue
		}
		if match := graphRole.FindStringSubmatch(line); match != nil {
			role = match[1]
			graph.Patterns[role] = nil
			continue
		}
		if match := graphSkill.FindStringSubmatch(line); match != nil && role != "" {
			graph.Patterns[role] = append(graph.Patterns[role], match[1])
		}
	}
	return graph
}

// ExpandRole resolves one role's patterns across every catalogue. A pattern
// that matches nothing is an error, since an empty selector hides a rename.
func ExpandRole(catalogs []string, role string, graph RoleGraph) (map[string]string, error) {
	admitted, _, err := ExpandRoleWithExclusions(catalogs, role, graph)
	return admitted, err
}

// ExpandRoleWithExclusions also reports what the deny list dropped, so a wider
// compile can show which sources it refused rather than only what it took.
func ExpandRoleWithExclusions(catalogs []string, role string, graph RoleGraph) (map[string]string, []string, error) {
	// A wider layer composes the same graph against catalogues the public build
	// cannot read, so the source of each name is an output, not an assumption.
	home := map[string]string{}
	available := make([]string, 0)
	for _, catalog := range catalogs {
		entries, err := os.ReadDir(filepath.Join(catalog, ".agents", "composed"))
		if err != nil {
			return nil, nil, fmt.Errorf("read composed catalogue %s: %w", catalog, err)
		}
		for _, entry := range entries {
			if !entry.IsDir() {
				continue
			}
			if previous, clash := home[entry.Name()]; clash {
				return nil, nil, fmt.Errorf(
					"skill %q is in both %s and %s; one catalogue must own it",
					entry.Name(), previous, catalog,
				)
			}
			home[entry.Name()] = catalog
			available = append(available, entry.Name())
		}
	}
	selected := map[string]struct{}{}
	excluded := []string{}
	for _, pattern := range graph.Patterns[role] {
		matched := 0
		for _, name := range available {
			ok, err := filepath.Match(pattern, name)
			if err != nil {
				return nil, nil, fmt.Errorf("role %q: bad pattern %q: %w", role, pattern, err)
			}
			if !ok {
				continue
			}
			if reason, denied := deniedReason(name); denied {
				// Naming one exactly asks for it. A family glob merely brushes
				// past, so the denied member drops out and the rest stands.
				if pattern == name {
					return nil, nil, fmt.Errorf("role %q: pattern %q is denied: %s", role, pattern, reason)
				}
				excluded = append(excluded, fmt.Sprintf("%s (via %q): %s", name, pattern, reason))
				matched++
				continue
			}
			selected[name] = struct{}{}
			matched++
		}
		if matched == 0 {
			return nil, nil, fmt.Errorf(
				"role %q: pattern %q matches nothing in %s",
				role, pattern, strings.Join(catalogs, ", "),
			)
		}
	}
	admitted := map[string]string{}
	for name := range selected {
		admitted[name] = home[name]
	}
	sort.Strings(excluded)
	return admitted, excluded, nil
}

// SortedNames orders an admitted set so callers render it deterministically.
func SortedNames(admitted map[string]string) []string {
	names := make([]string, 0, len(admitted))
	for name := range admitted {
		names = append(names, name)
	}
	sort.Strings(names)
	return names
}

// RenderDeclaration emits the source declaration agent-compose consumes. It is
// build output, so the tracked allowlist never carries a mechanical path.
func RenderDeclaration(id string, names []string) string {
	var out strings.Builder
	fmt.Fprintf(&out, "// Generated from agent/compose/roles.kdl. Do not edit.\nsource %q {\n", id)
	for _, name := range names {
		fmt.Fprintf(&out, "    skill %q path=\"skills/%s\"\n", name, name)
	}
	out.WriteString("}\n")
	return out.String()
}

// CheckGraphRoles refuses an allowlist entry for a role the roster does not
// have. See docs/sirens-echo-compose.md.
func CheckGraphRoles(graph RoleGraph, roster []string) error {
	known := make(map[string]bool, len(roster))
	for _, role := range roster {
		known[strings.TrimSpace(role)] = true
	}
	var orphans []string
	for role := range graph.Patterns {
		if !known[role] {
			orphans = append(orphans, role)
		}
	}
	if len(orphans) == 0 {
		return nil
	}
	sort.Strings(orphans)
	return fmt.Errorf(
		"role graph grants skills to %s, which the roster does not have: %s",
		strings.Join(orphans, ", "),
		strings.Join(roster, ", "),
	)
}

// checkGraphGlobals refuses a globalized private repository. The graph declares
// none today, and this keeps re-adding one from being quiet. See #126.
func CheckGraphGlobals(graph RoleGraph) error {
	for _, id := range graph.Globals {
		path, declared := graph.Repositories[id]
		if !declared {
			return fmt.Errorf("global %q names no declared repository", id)
		}
		if reason, private := PrivateRepositories[path]; private {
			return fmt.Errorf("global %q resolves to private %s: %s", id, path, reason)
		}
	}
	return nil
}

// VerifyDenyListResolves is the negative control the name-only tests cannot be,
// since those build their fixture from the list. sirens-echo#7615.
func VerifyDenyListResolves(catalogs []string) error {
	present := map[string]bool{}
	for _, catalog := range catalogs {
		entries, err := os.ReadDir(filepath.Join(catalog, ".agents", "composed"))
		if err != nil {
			return fmt.Errorf("read composed catalogue %s: %w", catalog, err)
		}
		for _, entry := range entries {
			if entry.IsDir() {
				present[entry.Name()] = true
			}
		}
	}

	problems := []string{}
	for name := range DeniedComposedSkills {
		if !present[name] {
			problems = append(problems, fmt.Sprintf(
				"denied skill %q resolves in no catalogue, so it guards nothing; "+
					"rename it to its current name or retire it", name))
		}
	}
	for name := range RetiredDeniedSkills {
		if present[name] {
			problems = append(problems, fmt.Sprintf(
				"retired skill %q is back in a catalogue; "+
					"move it to DeniedComposedSkills so it is verified", name))
		}
	}
	if len(problems) == 0 {
		return nil
	}
	sort.Strings(problems)
	return fmt.Errorf("deny list is stale against %s:\n  %s",
		strings.Join(catalogs, ", "), strings.Join(problems, "\n  "))
}
