package community

import (
	"fmt"
	"strings"
)

// A policy is read as intent and applied as effect, and the two diverge
// quietly. See docs/sirens-echo-access-check.md.

// inheritedTier is what an unset guild tier resolves to: the deployment's own,
// which this file cannot see and a reader must not assume is a bound.
const inheritedTier = "deployment default"

// RenderAccessSummary states what a loaded policy admits. It reads the resolved
// entries rather than the raw file, so it cannot disagree with the runtime.
func RenderAccessSummary(policy *AccessPolicy) string {
	if policy == nil {
		return "no access policy\n"
	}
	var out strings.Builder
	fmt.Fprintf(&out, "schema           %s\n", policy.Schema)
	fmt.Fprintf(&out, "denied accounts  %s\n", countOrNone(len(policy.Deny.Users)))
	fmt.Fprintf(&out, "direct messages  %s\n", countOrNone(len(policy.DirectMessages.Allow)))
	fmt.Fprintf(&out, "counterparts     %s\n", countOrNone(len(policy.Agents.Allow)))
	fmt.Fprintf(&out, "guilds           %s\n", countOrNone(len(policy.Guilds)))
	for index := range policy.Guilds {
		out.WriteString("\n")
		out.WriteString(renderGuildAccess(&policy.Guilds[index]))
	}
	return out.String()
}

// renderGuildAccess describes one guild. Snowflakes are counted rather than
// listed: the file already shows them and a count is what a diff reads.
func renderGuildAccess(guild *GuildAccess) string {
	var out strings.Builder
	fmt.Fprintf(&out, "guild %s%s\n", guild.ID, notedAs(guild.Note))
	fmt.Fprintf(&out, "  channels     %s\n", describeAllowlist(guild.Channels))
	fmt.Fprintf(&out, "  members      %s\n", describeAllowlist(guild.Users))
	fmt.Fprintf(&out, "  roles        %s\n", countOrNone(len(guild.Roles)))
	fmt.Fprintf(&out, "  staff roles  %s\n", countOrNone(len(guild.StaffRoles)))
	fmt.Fprintf(&out, "  per user     %s\n", describeTier(guild.Overrides(), tierPerUser))
	fmt.Fprintf(&out, "  per context  %s\n", describeTier(guild.Overrides(), tierPerContext))
	// The one combination validation refuses, restated as effect so a reader
	// sees why an open guild needs a bound rather than only that it has one.
	if guild.Users.All {
		out.WriteString("  every member of this guild is admitted\n")
	}
	return out.String()
}

type rateTier int

const (
	tierPerUser rateTier = iota
	tierPerContext
)

// describeTier reports the resolved bound. An absent override inherits, and an
// explicit `off` removes limiting, which must not read as inheriting.
func describeTier(override *RateLimitOverride, tier rateTier) string {
	if override == nil {
		return inheritedTier
	}
	limit := override.PerUser
	if tier == tierPerContext {
		limit = override.PerContext
	}
	if limit == nil {
		return inheritedTier
	}
	if !limit.enabled() {
		return "unlimited"
	}
	return fmt.Sprintf("%d per %s", limit.Burst, limit.Every)
}

func describeAllowlist(list Allowlist) string {
	if list.All {
		return "all"
	}
	return countOrNone(len(list.IDs))
}

func countOrNone(count int) string {
	if count == 0 {
		return "none"
	}
	return fmt.Sprintf("%d", count)
}

func notedAs(note string) string {
	if strings.TrimSpace(note) == "" {
		return ""
	}
	return " (" + note + ")"
}
