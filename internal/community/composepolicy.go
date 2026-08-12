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

// DeniedComposedSkills must never reach an agent that answers strangers. This
// is the invariant a glob is checked against, so it lives beside the expander
// rather than only in a test.
var DeniedComposedSkills = map[string]string{
	"kai-career":                 "private career context",
	"kai-job-search":             "private job search",
	"kai-grill-me":               "private operating context",
	"kai-collaboration":          "private collaboration context",
	"kai-kapwing-pr-review":      "employer team and domain context",
	"kai-linkedin-voice":         "a member's personal channel voice",
	"kai-linkedin-video":         "a member's personal channel format",
	"kai-bio-surface":            "resume and identity surface, points at private lore",
	"kai-public-repos":           "names private sibling repositories",
	"kai-engineering-voice":      "code review and eng-channel posts, not this agent",
	"kai-design-language":        "art direction, and low-context: required",
	"personal-preference-social": "an organization cannot own a person's social accounts",
	"tooling-cross-repo-infra":   "fleet mutation surface",
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

// ParseRoleGraph reads the tracked allowlist. It is line-oriented on purpose:
// the file is small, and a KDL dependency would buy nothing here.
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

// ExpandRole resolves one role's patterns against a catalogue. A pattern that
// matches nothing is an error: a silently empty selector is how an allowlist
// rots into permitting whatever the upstream rename produced.
func ExpandRole(catalog, role string, graph RoleGraph) ([]string, error) {
	entries, err := os.ReadDir(filepath.Join(catalog, ".agents", "composed"))
	if err != nil {
		return nil, fmt.Errorf("read composed catalogue: %w", err)
	}
	available := make([]string, 0, len(entries))
	for _, entry := range entries {
		if entry.IsDir() {
			available = append(available, entry.Name())
		}
	}
	selected := map[string]struct{}{}
	for _, pattern := range graph.Patterns[role] {
		matched := 0
		for _, name := range available {
			ok, err := filepath.Match(pattern, name)
			if err != nil {
				return nil, fmt.Errorf("role %q: bad pattern %q: %w", role, pattern, err)
			}
			if !ok {
				continue
			}
			if reason, denied := DeniedComposedSkills[name]; denied {
				return nil, fmt.Errorf("role %q: pattern %q reaches denied %q: %s", role, pattern, name, reason)
			}
			selected[name] = struct{}{}
			matched++
		}
		if matched == 0 {
			return nil, fmt.Errorf("role %q: pattern %q matches nothing in %s", role, pattern, catalog)
		}
	}
	names := make([]string, 0, len(selected))
	for name := range selected {
		names = append(names, name)
	}
	sort.Strings(names)
	return names, nil
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
