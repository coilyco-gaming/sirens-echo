package community

import (
	"fmt"
	"net/url"
	"regexp"
	"sort"
	"strings"
)

// A reply must not carry an identifier this process holds. Input framings are
// unbounded and output values are enumerable. See docs/sirens-echo-identifiers.md.

// snowflakePattern matches a Discord ID. A shorter digit run is an ordinary
// number a reply may legitimately contain, such as a port or a count.
var snowflakePattern = regexp.MustCompile(`^\d{17,20}$`)

// opaqueSecretRunes is the shortest opaque value worth guarding. Anything
// shorter is a word rather than a credential.
const opaqueSecretRunes = 20

// IdentifierGuard holds this process's own identifiers, admitted by shape
// rather than by having appeared in configuration.
type IdentifierGuard struct {
	forbidden []string
}

// NewIdentifierGuard derives the set from configuration at boot, so it cannot
// drift from what the pod actually holds. See docs/sirens-echo-identifiers.md.
func NewIdentifierGuard(
	cfg Config,
	policy *AccessPolicy,
	roster []MCPServerDefinition,
) *IdentifierGuard {
	guard := &IdentifierGuard{}
	// The principal ID reaches no tool that returns it, so it is forbidden
	// unconditionally rather than only when no tool ran.
	guard.addSnowflake(cfg.Principal.UserID)
	for _, channel := range cfg.DiscordChannelIDs {
		guard.addSnowflake(channel)
	}
	if policy != nil {
		for _, guild := range policy.Guilds {
			guard.addSnowflake(guild.ID)
			for _, channel := range guild.Channels.IDs {
				guard.addSnowflake(channel)
			}
		}
	}
	for _, server := range roster {
		guard.addEndpoint(server.URL)
	}
	guard.addEndpoint(cfg.AgentProxyURL)
	guard.addOpaque(cfg.DiscordToken)
	// Longest first, so a reported match names the most specific value rather
	// than a host that happens to prefix an endpoint.
	sort.Slice(guard.forbidden, func(a, b int) bool {
		return len(guard.forbidden[a]) > len(guard.forbidden[b])
	})
	return guard
}

// The handle is deliberately absent. It is a substring of a host tool output
// legitimately returns, and ValidateIdentityClaim already owns it.

func (g *IdentifierGuard) addSnowflake(value string) {
	value = strings.TrimSpace(value)
	if snowflakePattern.MatchString(value) {
		g.add(value)
	}
}

// addEndpoint keeps the host and port together. A bare host is a public name
// and a bare port is an ordinary number, so neither is guarded alone.
func (g *IdentifierGuard) addEndpoint(raw string) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return
	}
	parsed, err := url.Parse(raw)
	if err != nil || parsed.Host == "" || parsed.Port() == "" {
		return
	}
	g.add(parsed.Host)
}

func (g *IdentifierGuard) addOpaque(value string) {
	value = strings.TrimSpace(value)
	if len([]rune(value)) >= opaqueSecretRunes {
		g.add(value)
	}
}

func (g *IdentifierGuard) add(value string) {
	for _, existing := range g.forbidden {
		if existing == value {
			return
		}
	}
	g.forbidden = append(g.forbidden, value)
}

// Guarded reports how many identifiers the set holds, so a deployment can see
// the guard is populated without the values reaching a log.
func (g *IdentifierGuard) Guarded() int {
	if g == nil {
		return 0
	}
	return len(g.forbidden)
}

// Validate rejects a reply carrying one of this process's identifiers. The
// error names the class rather than the value, which must not reach a log.
func (g *IdentifierGuard) Validate(reply string) error {
	if g == nil || len(g.forbidden) == 0 {
		return nil
	}
	for _, value := range g.forbidden {
		if strings.Contains(reply, value) {
			return fmt.Errorf("model reply carried a configured identifier")
		}
	}
	return nil
}
