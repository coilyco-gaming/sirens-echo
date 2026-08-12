package community

import (
	"fmt"
	"os"
	"strings"

	"gopkg.in/yaml.v3"
)

// The access policy is the deployment-owned answer to who may summon this
// service. See docs/sirens-echo-access.md.

const accessPolicySchema = "coilyco-harness.access.v1"

// accessReason is the closed set of admission-gate outcomes. It is a metric
// label, so it never carries a guild, channel, or member identifier.
type accessReason string

const (
	accessAllowed        accessReason = "allowed"
	accessDeniedBlocked  accessReason = "denied_blocked"
	accessDeniedDM       accessReason = "denied_dm"
	accessDeniedGuild    accessReason = "denied_guild"
	accessDeniedChannel  accessReason = "denied_channel"
	accessDeniedMember   accessReason = "denied_member"
	accessNeedsThreadRef accessReason = "needs_thread_lookup"
	accessDeniedExchange accessReason = "denied_agent_exchange"
)

// Allowlist is either the literal `all` or an explicit ID list. See
// docs/sirens-echo-access.md.
type Allowlist struct {
	All bool
	IDs []string
}

// UnmarshalYAML accepts the scalar `all` or a sequence of ID strings.
func (a *Allowlist) UnmarshalYAML(value *yaml.Node) error {
	if value.Kind == yaml.ScalarNode {
		if strings.TrimSpace(value.Value) != "all" {
			return fmt.Errorf("expected `all` or a list of IDs, got %q", value.Value)
		}
		a.All = true
		return nil
	}
	if value.Kind != yaml.SequenceNode {
		return fmt.Errorf("expected `all` or a list of IDs")
	}
	return value.Decode(&a.IDs)
}

// Permits reports whether an ID is covered by this allowlist.
func (a Allowlist) Permits(id string) bool {
	if a.All {
		return true
	}
	for _, candidate := range a.IDs {
		if candidate == id {
			return true
		}
	}
	return false
}

func (a Allowlist) empty() bool { return !a.All && len(a.IDs) == 0 }

// GuildRateLimit optionally tightens admission for one guild. An unset tier
// keeps the deployment default.
type GuildRateLimit struct {
	PerUser    string `yaml:"per_user"`
	PerContext string `yaml:"per_context"`
}

// GuildAccess is one guild's entry. Note is a review aid for humans reading a
// diff of snowflakes and is never matched on.
type GuildAccess struct {
	ID       string          `yaml:"id"`
	Note     string          `yaml:"note"`
	Channels Allowlist       `yaml:"channels"`
	Users    Allowlist       `yaml:"users"`
	Roles    []string        `yaml:"roles"`
	Rate     *GuildRateLimit `yaml:"rate_limit"`

	// catchAll marks the entry synthesized from the legacy environment, which
	// scoped by channel across every guild. No file can author one.
	catchAll  bool
	overrides *RateLimitOverride
}

// DenyList is evaluated before every allow rule, so one abusive member can be
// stopped without withdrawing the whole guild.
type DenyList struct {
	Users []string `yaml:"users"`
}

// DirectMessageAccess allowlists the accounts whose direct messages are
// answered. An absent or empty list answers none.
type DirectMessageAccess struct {
	Allow []string `yaml:"allow"`
}

// AgentAccess allowlists the counterpart agents whose messages are answered.
// Empty answers none, which is the shipped posture.
type AgentAccess struct {
	Allow []string `yaml:"allow"`
}

// AccessPolicy is the deployment-owned allowlist for one process.
type AccessPolicy struct {
	Schema         string              `yaml:"schema"`
	Deny           DenyList            `yaml:"deny"`
	DirectMessages DirectMessageAccess `yaml:"direct_messages"`
	Agents         AgentAccess         `yaml:"agents"`
	Guilds         []GuildAccess       `yaml:"guilds"`

	byGuild  map[string]*GuildAccess
	catchAll *GuildAccess
	// legacyOpenDMs preserves the environment switch, which had no per-account
	// list. The file schema has no way to express it.
	legacyOpenDMs bool
}

// accessDecision is the gate's verdict for one summon.
type accessDecision struct {
	Reason accessReason
	// Guild is the matched entry, present whenever a guild matched at all.
	Guild *GuildAccess
}

func (d accessDecision) allowed() bool { return d.Reason == accessAllowed }

// LoadAccessPolicy reads and validates the deployment's allowlist file.
func LoadAccessPolicy(path string) (*AccessPolicy, error) {
	raw, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read access policy: %w", err)
	}
	var policy AccessPolicy
	decoder := yaml.NewDecoder(strings.NewReader(string(raw)))
	decoder.KnownFields(true)
	if err := decoder.Decode(&policy); err != nil {
		return nil, fmt.Errorf("parse access policy: %w", err)
	}
	if err := policy.validate(); err != nil {
		return nil, err
	}
	return &policy, nil
}

// validate fails closed. A malformed or empty policy must stop the process
// rather than widen the surface it was written to narrow.
func (p *AccessPolicy) validate() error {
	if p.Schema != accessPolicySchema {
		return fmt.Errorf("unsupported access policy schema %q", p.Schema)
	}
	if len(p.Guilds) == 0 && len(p.DirectMessages.Allow) == 0 {
		return fmt.Errorf("access policy grants nothing: add a guild or a direct_messages allow entry")
	}
	ids := append(append([]string{}, p.Deny.Users...), p.DirectMessages.Allow...)
	for _, id := range append(ids, p.Agents.Allow...) {
		if !discordSnowflake.MatchString(id) {
			return fmt.Errorf("access policy IDs must be numeric snowflakes, got %q", id)
		}
	}
	p.byGuild = make(map[string]*GuildAccess, len(p.Guilds))
	for index := range p.Guilds {
		guild := &p.Guilds[index]
		if err := guild.validate(); err != nil {
			return err
		}
		if _, exists := p.byGuild[guild.ID]; exists {
			return fmt.Errorf("access policy lists guild %q twice", guild.ID)
		}
		p.byGuild[guild.ID] = guild
	}
	return nil
}

func (g *GuildAccess) validate() error {
	ids := append([]string{g.ID}, g.Roles...)
	if !g.Channels.All {
		ids = append(ids, g.Channels.IDs...)
	}
	if !g.Users.All {
		ids = append(ids, g.Users.IDs...)
	}
	for _, id := range ids {
		if !discordSnowflake.MatchString(id) {
			return fmt.Errorf("access policy IDs must be numeric snowflakes, got %q", id)
		}
	}
	if g.Channels.empty() {
		return fmt.Errorf("guild %q allows no channel: use `all` or list channel IDs", g.ID)
	}
	// Users and roles are a union, so either one alone is a complete grant.
	if g.Users.empty() && len(g.Roles) == 0 {
		return fmt.Errorf("guild %q allows no member: use `all`, list users, or list roles", g.ID)
	}
	return g.resolveOverrides()
}

func (g *GuildAccess) resolveOverrides() error {
	if g.Rate == nil {
		return nil
	}
	override := &RateLimitOverride{}
	for _, tier := range []struct {
		raw    string
		target **RateLimit
	}{
		{g.Rate.PerUser, &override.PerUser},
		{g.Rate.PerContext, &override.PerContext},
	} {
		limit, ok, err := parseRateLimit(tier.raw)
		if err != nil {
			return fmt.Errorf("guild %q rate_limit: %w", g.ID, err)
		}
		if ok {
			value := limit
			*tier.target = &value
		}
	}
	g.overrides = override
	return nil
}

// Overrides returns the per-guild admission overrides, or nil for the
// deployment defaults.
func (g *GuildAccess) Overrides() *RateLimitOverride {
	if g == nil {
		return nil
	}
	return g.overrides
}

// guild resolves a guild ID to its entry, falling back to the synthesized
// catch-all when the legacy environment configuration is in use.
func (p *AccessPolicy) guild(id string) *GuildAccess {
	if entry, exists := p.byGuild[id]; exists {
		return entry
	}
	return p.catchAll
}

// Evaluate applies the gate in cost order. A thread whose parent is unknown
// returns accessNeedsThreadRef so the caller decides whether to spend a lookup.
func (p *AccessPolicy) Evaluate(
	origin summonContext,
	userID string,
	roles []string,
	knownChannels func(string) bool,
) accessDecision {
	for _, blocked := range p.Deny.Users {
		if blocked == userID {
			return accessDecision{Reason: accessDeniedBlocked}
		}
	}
	if origin.Kind == contextKindDM {
		if p.legacyOpenDMs {
			return accessDecision{Reason: accessAllowed}
		}
		for _, allowed := range p.DirectMessages.Allow {
			if allowed == userID {
				return accessDecision{Reason: accessAllowed}
			}
		}
		return accessDecision{Reason: accessDeniedDM}
	}
	guild := p.guild(origin.GuildID)
	if guild == nil {
		return accessDecision{Reason: accessDeniedGuild}
	}
	if !guild.permitsMember(userID, roles) {
		return accessDecision{Reason: accessDeniedMember, Guild: guild}
	}
	switch {
	case guild.Channels.Permits(origin.ChannelID):
		return accessDecision{Reason: accessAllowed, Guild: guild}
	case knownChannels != nil && knownChannels(origin.ChannelID):
		return accessDecision{Reason: accessAllowed, Guild: guild}
	default:
		return accessDecision{Reason: accessNeedsThreadRef, Guild: guild}
	}
}

// permitsMember unions the user and role allowlists, so a role grant covers
// members the operator has never enumerated.
func (g *GuildAccess) permitsMember(userID string, roles []string) bool {
	if g.Users.Permits(userID) {
		return true
	}
	for _, granted := range g.Roles {
		for _, held := range roles {
			if granted == held {
				return true
			}
		}
	}
	return false
}

// PermitsAgent reports whether a counterpart agent is answered at all. A bot
// account is refused unless the deployment named it.
func (p *AccessPolicy) PermitsAgent(userID string) bool {
	if p == nil {
		return false
	}
	for _, allowed := range p.Agents.Allow {
		if allowed == userID {
			return true
		}
	}
	return false
}

// PermitsChannel reports whether a resolved thread parent is in scope.
func (g *GuildAccess) PermitsChannel(id string) bool {
	if g == nil {
		return false
	}
	return g.Channels.Permits(id)
}

// synthesizeAccessPolicy builds the equivalent policy for a deployment still
// configured through environment variables, keeping the gate single-pathed.
func synthesizeAccessPolicy(cfg Config) *AccessPolicy {
	channels := Allowlist{IDs: cfg.DiscordChannelIDs}
	policy := &AccessPolicy{
		Schema:  accessPolicySchema,
		byGuild: make(map[string]*GuildAccess, len(cfg.DiscordGuildIDs)),
	}
	policy.legacyOpenDMs = cfg.DiscordDMEnabled
	if len(cfg.DiscordGuildIDs) == 0 {
		policy.catchAll = &GuildAccess{
			Channels: channels,
			Users:    Allowlist{All: true},
			catchAll: true,
		}
		return policy
	}
	policy.Guilds = make([]GuildAccess, 0, len(cfg.DiscordGuildIDs))
	for _, id := range cfg.DiscordGuildIDs {
		policy.Guilds = append(policy.Guilds, GuildAccess{
			ID:       id,
			Channels: channels,
			Users:    Allowlist{All: true},
		})
	}
	for index := range policy.Guilds {
		policy.byGuild[policy.Guilds[index].ID] = &policy.Guilds[index]
	}
	return policy
}
