package community

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"os"
	"regexp"
	"sort"
	"strings"
)

// The agent's knowledge of its own authority is generated from the guardfile,
// never written by hand. See docs/sirens-echo-config.md.

// GuardGrant is one `can <verb> <resource>` block.
type GuardGrant struct {
	Verb     string
	Resource string
	Path     string
}

// Name is the model-facing tool name the MCP publishes for this grant.
func (g GuardGrant) Name() string {
	return g.Verb + "_" + g.Resource
}

// Guardfile is the parsed boundary: what is granted, and against what.
type Guardfile struct {
	Server string
	// Repository is the single repository every path is fixed to, or empty if
	// the paths do not agree, which is itself worth reporting.
	Repository string
	Grants     []GuardGrant
	// Digest identifies the exact source, so a stale skill is detectable.
	Digest string
}

var (
	guardServer   = regexp.MustCompile(`^\s*wrap\s+ward\s+mcp\s+([a-z0-9-]+)\s*\{`)
	guardCan      = regexp.MustCompile(`^\s*can\s+([a-z-]+)\s+([a-z-]+)\s*\{`)
	guardPath     = regexp.MustCompile(`^\s*path\s+"([^"]+)"`)
	guardRepoPath = regexp.MustCompile(`^/repos/([^/]+/[^/]+)/`)
)

// ParseGuardfile reads the KDL boundary. It is line-oriented on purpose: the
// file is small and a KDL dependency would buy nothing.
func ParseGuardfile(path string) (Guardfile, error) {
	raw, err := os.ReadFile(path)
	if err != nil {
		return Guardfile{}, fmt.Errorf("read guardfile: %w", err)
	}
	sum := sha256.Sum256(raw)
	parsed := Guardfile{Digest: hex.EncodeToString(sum[:8])}
	repositories := make(map[string]struct{})
	var pending *GuardGrant
	for _, line := range strings.Split(string(raw), "\n") {
		if match := guardServer.FindStringSubmatch(line); match != nil {
			parsed.Server = match[1]
			continue
		}
		if match := guardCan.FindStringSubmatch(line); match != nil {
			if pending != nil {
				parsed.Grants = append(parsed.Grants, *pending)
			}
			pending = &GuardGrant{Verb: match[1], Resource: match[2]}
			continue
		}
		if match := guardPath.FindStringSubmatch(line); match != nil && pending != nil {
			pending.Path = match[1]
			if repo := guardRepoPath.FindStringSubmatch(match[1]); repo != nil {
				repositories[repo[1]] = struct{}{}
			}
		}
	}
	if pending != nil {
		parsed.Grants = append(parsed.Grants, *pending)
	}
	if len(parsed.Grants) == 0 {
		return Guardfile{}, fmt.Errorf("guardfile %s declares no grant", path)
	}
	if len(repositories) == 1 {
		for repository := range repositories {
			parsed.Repository = repository
		}
	}
	sort.Slice(parsed.Grants, func(i, j int) bool {
		return parsed.Grants[i].Name() < parsed.Grants[j].Name()
	})
	return parsed, nil
}

// deniedByAbsence names what the surrounding API has and this guardfile does
// not. See docs/sirens-echo-config.md.
var deniedByAbsence = []string{
	"edit an issue body",
	"edit or delete a comment",
	"delete an issue",
	"reopen a closed issue",
	"pin an issue",
	"create or edit a label",
	"read or write a pull request",
	"read or write a release or tag",
	"read or write repository, organization, or account settings",
	"reach any repository other than the one every path is fixed to",
}

// RenderGuardfileSkill writes the agent's description of its own authority.
func RenderGuardfileSkill(guard Guardfile) string {
	var output strings.Builder
	output.WriteString("# The authority I actually hold\n\n")
	output.WriteString("Generated from the deployed guardfile by `just guardfile-skill`.\n")
	output.WriteString("Do not edit by hand: a hand-written description of a boundary drifts\n")
	output.WriteString("from the boundary, and describing my own limits wrongly is worse than\n")
	output.WriteString("declining to describe them.\n\n")
	fmt.Fprintf(&output, "Source digest: %s\n\n", guard.Digest)

	output.WriteString("## What I can do\n\n")
	if guard.Repository != "" {
		fmt.Fprintf(&output,
			"Every path below is fixed to `%s`.\nThere is no owner or repository argument, so I cannot be pointed at\nanother repository by asking.\n\n",
			guard.Repository)
	}
	for _, grant := range guard.Grants {
		fmt.Fprintf(&output, "* `%s` - %s\n", grant.Name(), grant.Path)
	}

	output.WriteString("\n## What I cannot do, and why\n\n")
	output.WriteString("The list above is exhaustive. Anything absent from it is denied by\n")
	output.WriteString("absence rather than by a rule saying no, which means there is no\n")
	output.WriteString("exception to find and nothing to argue with. I cannot:\n\n")
	for _, denied := range deniedByAbsence {
		fmt.Fprintf(&output, "* %s\n", denied)
	}
	output.WriteString("\n## How to answer questions about this\n\n")
	output.WriteString("Say what the grants are and say that everything else is absent rather\n")
	output.WriteString("than forbidden. If asked about something not listed, say I do not have\n")
	output.WriteString("it rather than guessing whether it would be allowed.\n")
	return output.String()
}
