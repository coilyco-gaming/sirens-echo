package community

import (
	"fmt"
	"net/url"
	"os"
	"regexp"
	"strconv"
	"strings"
	"time"

	"gopkg.in/yaml.v3"
)

const (
	defaultDefinitionPath = "agent/sirens-echo.yaml"
	defaultHTTPListenAddr = "127.0.0.1:8080"
	defaultInstanceName   = "sirens-echo"

	ResponseStyleNeutral = "neutral"
	ResponseStyleSocial  = "social"
)

// Admission defaults are sized for a guild the operator does not moderate.
// See docs/sirens-echo-admission.md.
const (
	defaultRequestTimeout = 3 * time.Minute
	// defaultQueueTimeout bounds the wait for the execution slot. A longer
	// wait answers a conversation that has already moved on.
	defaultQueueTimeout = 30 * time.Second
)

var defaultRateLimitPolicy = RateLimitPolicy{
	PerUser:     RateLimit{Burst: 3, Every: 30 * time.Second},
	PerContext:  RateLimit{Burst: 10, Every: 10 * time.Second},
	Global:      RateLimit{Burst: 20, Every: 5 * time.Second},
	MaxPending:  8,
	NotifyEvery: 5 * time.Minute,
}

// DefaultAgentProxyURL is a neutral fallback. Deployment owns the real
// endpoint and sets AGENT_PROXY_URL.
const DefaultAgentProxyURL = "http://agent-proxy:8080"

// DefaultOTLPEndpoint is the existing in-cluster SigNoZ collector.
const DefaultOTLPEndpoint = "http://signoz-otel-collector.observability.svc.cluster.local:4318"

var (
	mcpServerNamePattern   = regexp.MustCompile(`^[a-z][a-z0-9-]*$`)
	environmentNamePattern = regexp.MustCompile(`^[A-Z][A-Z0-9_]*$`)
	discordSnowflake       = regexp.MustCompile(`^[0-9]{15,20}$`)
	discordHandlePattern   = regexp.MustCompile(`^[a-z0-9._]{2,32}$`)
	// channelLabelPattern matches the grounding validator's channel form, so a
	// label cannot introduce a reference the model is rejected for repeating.
	channelLabelPattern = regexp.MustCompile(`^#[A-Za-z_][A-Za-z0-9_-]*$`)
	// rateLimitPattern is "<burst>/<refill interval for one token>".
	rateLimitPattern = regexp.MustCompile(`^([0-9]+)/(.+)$`)
)

// MCPServerDefinition is one model tool surface selected for Sirens Echo.
type MCPServerDefinition struct {
	Name   string `json:"name" yaml:"name"`
	URL    string `json:"url,omitempty" yaml:"url,omitempty"`
	URLEnv string `json:"url_env,omitempty" yaml:"url_env,omitempty"`
}

// Definition is the source-controlled attribution, route, and policy selection.
type Definition struct {
	Schema             string                `json:"schema" yaml:"schema"`
	Identity           string                `json:"identity" yaml:"identity"`
	AuditRole          string                `json:"audit_role" yaml:"audit_role"`
	ResponseStyle      string                `json:"response_style" yaml:"response_style"`
	Channel            string                `json:"channel" yaml:"channel"`
	MaxContextMessages int                   `json:"max_context_messages" yaml:"max_context_messages"`
	LocalSkillRoots    []string              `json:"local_skill_roots" yaml:"local_skill_roots"`
	MCPServers         []MCPServerDefinition `json:"mcp_servers" yaml:"mcp_servers"`
	IssueTracker       string                `json:"issue_tracker,omitempty" yaml:"issue_tracker,omitempty"`
}

// Principal identifies the one speaker the prompt trusts. The values are
// deployment-owned. See docs/sirens-echo-prompt.md.
type Principal struct {
	Handle string
	UserID string
}

// Configured reports whether deployment supplied both signals. One alone
// identifies nobody, so the prompt renders neither.
func (p Principal) Configured() bool { return p.Handle != "" && p.UserID != "" }

// PlaceholderPrincipal renders the tracked snapshot and the build-time policy
// check. It is not a real account, matching docs/access-policy.reference.yaml.
var PlaceholderPrincipal = Principal{Handle: "example_handle", UserID: "1024000000000000001"}

// Config combines the source-controlled definition with deployment secrets.
type Config struct {
	Definition     Definition
	DefinitionPath string
	InstanceName   string
	// Principal is empty until deployment names the trusted account.
	Principal      Principal
	DiscordEnabled bool
	DiscordToken   string
	// DiscordChannelIDs are the channels that may summon this deployment, plus
	// their threads. Channel IDs are globally unique, so the list spans guilds.
	DiscordChannelIDs []string
	// DiscordGuildIDs optionally restricts which guilds may summon at all.
	// Empty means every guild the bot joined, still bounded by the channels.
	DiscordGuildIDs []string
	// DiscordDMEnabled admits direct messages. Off by default, because a
	// direct message has no guild moderation behind it.
	DiscordDMEnabled bool
	AgentProxyURL    string
	AgentProxyModel  string
	OTLPEndpoint     string
	HTTPListenAddr   string
	// AccessPolicyPath names the deployment's tracked allowlist file. Empty
	// synthesizes the equivalent from the Discord environment variables.
	AccessPolicyPath string
	RequestTimeout   time.Duration
	QueueTimeout     time.Duration
	RateLimit        RateLimitPolicy
}

// LoadConfig loads the Sirens Echo deployment from environment and its
// source-controlled definition. Secrets never have source defaults.
func LoadConfig() (Config, error) {
	definitionPath := valueOrDefault(os.Getenv("SIRENS_ECHO_DEFINITION"), defaultDefinitionPath)
	definition, err := LoadDefinition(definitionPath)
	if err != nil {
		return Config{}, err
	}
	discordEnabled, err := boolOrDefault(os.Getenv("SIRENS_ECHO_DISCORD_ENABLED"), true)
	if err != nil {
		return Config{}, fmt.Errorf("SIRENS_ECHO_DISCORD_ENABLED: %w", err)
	}
	dmEnabled, err := boolOrDefault(os.Getenv("SIRENS_ECHO_DISCORD_DM_ENABLED"), false)
	if err != nil {
		return Config{}, fmt.Errorf("SIRENS_ECHO_DISCORD_DM_ENABLED: %w", err)
	}
	requestTimeout, err := durationOrDefault(
		os.Getenv("SIRENS_ECHO_REQUEST_TIMEOUT"),
		defaultRequestTimeout,
	)
	if err != nil {
		return Config{}, fmt.Errorf("SIRENS_ECHO_REQUEST_TIMEOUT: %w", err)
	}
	queueTimeout, err := durationOrDefault(
		os.Getenv("SIRENS_ECHO_QUEUE_TIMEOUT"),
		defaultQueueTimeout,
	)
	if err != nil {
		return Config{}, fmt.Errorf("SIRENS_ECHO_QUEUE_TIMEOUT: %w", err)
	}
	rateLimit, err := loadRateLimitPolicy()
	if err != nil {
		return Config{}, err
	}
	cfg := Config{
		Definition:     definition,
		DefinitionPath: definitionPath,
		InstanceName:   valueOrDefault(os.Getenv("SIRENS_ECHO_INSTANCE"), defaultInstanceName),
		Principal: Principal{
			Handle: strings.TrimSpace(os.Getenv("SIRENS_ECHO_PRINCIPAL_HANDLE")),
			UserID: strings.TrimSpace(os.Getenv("SIRENS_ECHO_PRINCIPAL_USER_ID")),
		},
		DiscordEnabled:    discordEnabled,
		DiscordToken:      strings.TrimSpace(os.Getenv("DISCORD_TOKEN")),
		DiscordChannelIDs: splitList(os.Getenv("DISCORD_CHANNEL_ID")),
		DiscordGuildIDs:   splitList(os.Getenv("DISCORD_GUILD_IDS")),
		DiscordDMEnabled:  dmEnabled,
		AgentProxyURL:     valueOrDefault(os.Getenv("AGENT_PROXY_URL"), DefaultAgentProxyURL),
		AgentProxyModel:   strings.TrimSpace(os.Getenv("AGENT_PROXY_MODEL")),
		OTLPEndpoint:      valueOrDefault(os.Getenv("OTEL_EXPORTER_OTLP_ENDPOINT"), DefaultOTLPEndpoint),
		HTTPListenAddr:    valueOrDefault(os.Getenv("SIRENS_ECHO_HTTP_ADDR"), defaultHTTPListenAddr),
		AccessPolicyPath:  strings.TrimSpace(os.Getenv("SIRENS_ECHO_ACCESS_POLICY")),
		RequestTimeout:    requestTimeout,
		QueueTimeout:      queueTimeout,
		RateLimit:         rateLimit,
	}
	if !mcpServerNamePattern.MatchString(cfg.InstanceName) {
		return Config{}, fmt.Errorf("SIRENS_ECHO_INSTANCE must be a lowercase service name")
	}
	if err := validatePrincipal(cfg.Principal); err != nil {
		return Config{}, err
	}
	for _, id := range append(append([]string{}, cfg.DiscordChannelIDs...), cfg.DiscordGuildIDs...) {
		if !discordSnowflake.MatchString(id) {
			return Config{}, fmt.Errorf("Discord IDs must be numeric snowflakes, got %q", id)
		}
	}
	missing := make([]string, 0, 5)
	if cfg.DiscordEnabled {
		if cfg.DiscordToken == "" {
			missing = append(missing, "DISCORD_TOKEN")
		}
		// The access policy file supplies scope on its own. Otherwise a
		// channel list is required unless direct messages are the only ingress.
		if cfg.AccessPolicyPath == "" &&
			len(cfg.DiscordChannelIDs) == 0 && !cfg.DiscordDMEnabled {
			missing = append(missing, "DISCORD_CHANNEL_ID")
		}
	}
	if cfg.AgentProxyModel == "" {
		missing = append(missing, "AGENT_PROXY_MODEL")
	}
	for index := range cfg.Definition.MCPServers {
		server := &cfg.Definition.MCPServers[index]
		if server.URLEnv == "" {
			continue
		}
		server.URL = strings.TrimSpace(os.Getenv(server.URLEnv))
		if server.URL == "" {
			missing = append(missing, server.URLEnv)
			continue
		}
		if !validHTTPURL(server.URL) {
			return Config{}, fmt.Errorf("MCP server %q has invalid URL from %s", server.Name, server.URLEnv)
		}
	}
	if len(missing) > 0 {
		return Config{}, fmt.Errorf("missing required env: %v", missing)
	}
	return cfg, nil
}

// validatePrincipal rejects a half-configured principal, which would otherwise
// render a sentence naming one signal and an empty string for the other.
func validatePrincipal(principal Principal) error {
	if principal.Handle == "" && principal.UserID == "" {
		return nil
	}
	if !principal.Configured() {
		return fmt.Errorf(
			"SIRENS_ECHO_PRINCIPAL_HANDLE and SIRENS_ECHO_PRINCIPAL_USER_ID must be set together",
		)
	}
	if !discordHandlePattern.MatchString(principal.Handle) {
		return fmt.Errorf(
			"SIRENS_ECHO_PRINCIPAL_HANDLE must be a Discord username, got %q",
			principal.Handle,
		)
	}
	if !discordSnowflake.MatchString(principal.UserID) {
		return fmt.Errorf(
			"SIRENS_ECHO_PRINCIPAL_USER_ID must be a numeric snowflake, got %q",
			principal.UserID,
		)
	}
	return nil
}

// LoadDefinition reads and validates the repository-owned agent definition.
func LoadDefinition(path string) (Definition, error) {
	raw, err := os.ReadFile(path)
	if err != nil {
		return Definition{}, fmt.Errorf("read agent definition: %w", err)
	}
	var definition Definition
	if err := yaml.Unmarshal(raw, &definition); err != nil {
		return Definition{}, fmt.Errorf("parse agent definition: %w", err)
	}
	if definition.Schema != "coilyco-harness.agent.v1" {
		return Definition{}, fmt.Errorf("unsupported agent definition schema %q", definition.Schema)
	}
	if definition.Identity == "" || definition.AuditRole == "" {
		return Definition{}, fmt.Errorf("agent definition requires identity and audit_role")
	}
	// Channel is the prompt's boundary label, not the routing key. Deployment
	// owns routing through DISCORD_CHANNEL_ID.
	if definition.Channel != "" && !channelLabelPattern.MatchString(definition.Channel) {
		return Definition{}, fmt.Errorf(
			"agent definition channel must be empty or a #channel-name, got %q",
			definition.Channel,
		)
	}
	if definition.ResponseStyle != ResponseStyleNeutral &&
		definition.ResponseStyle != ResponseStyleSocial {
		return Definition{}, fmt.Errorf("unsupported response_style %q", definition.ResponseStyle)
	}
	if definition.MaxContextMessages < 1 || definition.MaxContextMessages > 50 {
		return Definition{}, fmt.Errorf("max_context_messages must be between 1 and 50")
	}
	if len(definition.LocalSkillRoots) == 0 {
		return Definition{}, fmt.Errorf("agent definition requires at least one local skill root")
	}
	seenServers := make(map[string]struct{}, len(definition.MCPServers))
	for _, server := range definition.MCPServers {
		if !mcpServerNamePattern.MatchString(server.Name) {
			return Definition{}, fmt.Errorf("invalid MCP server name %q", server.Name)
		}
		if _, exists := seenServers[server.Name]; exists {
			return Definition{}, fmt.Errorf("duplicate MCP server %q", server.Name)
		}
		seenServers[server.Name] = struct{}{}
		hasURL := strings.TrimSpace(server.URL) != ""
		hasURLEnv := strings.TrimSpace(server.URLEnv) != ""
		if hasURL == hasURLEnv {
			return Definition{}, fmt.Errorf("MCP server %q requires exactly one of url or url_env", server.Name)
		}
		if hasURL && !validHTTPURL(server.URL) {
			return Definition{}, fmt.Errorf("MCP server %q has invalid URL", server.Name)
		}
		if hasURLEnv && !environmentNamePattern.MatchString(server.URLEnv) {
			return Definition{}, fmt.Errorf("MCP server %q has invalid url_env", server.Name)
		}
	}
	if definition.IssueTracker != "" {
		if _, exists := seenServers[definition.IssueTracker]; !exists {
			return Definition{}, fmt.Errorf(
				"issue_tracker %q does not name a configured MCP server",
				definition.IssueTracker,
			)
		}
	}
	return definition, nil
}

func validHTTPURL(value string) bool {
	parsed, err := url.Parse(value)
	return err == nil && (parsed.Scheme == "http" || parsed.Scheme == "https") && parsed.Host != ""
}

func valueOrDefault(value, fallback string) string {
	if trimmed := strings.TrimSpace(value); trimmed != "" {
		return trimmed
	}
	return fallback
}

// splitList parses a comma-separated deployment list, dropping empty entries so
// a trailing comma is not a configuration error.
func splitList(value string) []string {
	items := make([]string, 0, 4)
	for _, part := range strings.Split(value, ",") {
		if trimmed := strings.TrimSpace(part); trimmed != "" {
			items = append(items, trimmed)
		}
	}
	return items
}

func durationOrDefault(value string, fallback time.Duration) (time.Duration, error) {
	trimmed := strings.TrimSpace(value)
	if trimmed == "" {
		return fallback, nil
	}
	parsed, err := time.ParseDuration(trimmed)
	if err != nil {
		return 0, fmt.Errorf("must be a Go duration such as 90s")
	}
	if parsed <= 0 {
		return 0, fmt.Errorf("must be greater than zero")
	}
	return parsed, nil
}

// loadRateLimitPolicy overlays deployment overrides onto the packaged
// defaults. See docs/sirens-echo-admission.md for the format.
func loadRateLimitPolicy() (RateLimitPolicy, error) {
	policy := defaultRateLimitPolicy
	tiers := []struct {
		name   string
		target *RateLimit
	}{
		{"SIRENS_ECHO_RATE_USER", &policy.PerUser},
		{"SIRENS_ECHO_RATE_CONTEXT", &policy.PerContext},
		{"SIRENS_ECHO_RATE_GLOBAL", &policy.Global},
	}
	for _, tier := range tiers {
		limit, ok, err := parseRateLimit(os.Getenv(tier.name))
		if err != nil {
			return RateLimitPolicy{}, fmt.Errorf("%s: %w", tier.name, err)
		}
		if ok {
			*tier.target = limit
		}
	}
	pending, err := intOrDefault(os.Getenv("SIRENS_ECHO_MAX_PENDING"), policy.MaxPending)
	if err != nil {
		return RateLimitPolicy{}, fmt.Errorf("SIRENS_ECHO_MAX_PENDING: %w", err)
	}
	policy.MaxPending = pending
	notify, err := durationOrDefault(
		os.Getenv("SIRENS_ECHO_RATE_NOTIFY_EVERY"),
		policy.NotifyEvery,
	)
	if err != nil {
		return RateLimitPolicy{}, fmt.Errorf("SIRENS_ECHO_RATE_NOTIFY_EVERY: %w", err)
	}
	policy.NotifyEvery = notify
	return policy, nil
}

func parseRateLimit(value string) (RateLimit, bool, error) {
	trimmed := strings.TrimSpace(value)
	if trimmed == "" {
		return RateLimit{}, false, nil
	}
	if trimmed == "off" {
		return RateLimit{}, true, nil
	}
	match := rateLimitPattern.FindStringSubmatch(trimmed)
	if match == nil {
		return RateLimit{}, false, fmt.Errorf(`must be "<burst>/<interval>" such as 3/30s, or "off"`)
	}
	burst, err := strconv.Atoi(match[1])
	if err != nil || burst < 1 {
		return RateLimit{}, false, fmt.Errorf("burst must be a positive integer")
	}
	every, err := time.ParseDuration(match[2])
	if err != nil || every <= 0 {
		return RateLimit{}, false, fmt.Errorf("interval must be a positive Go duration such as 30s")
	}
	return RateLimit{Burst: burst, Every: every}, true, nil
}

func intOrDefault(value string, fallback int) (int, error) {
	trimmed := strings.TrimSpace(value)
	if trimmed == "" {
		return fallback, nil
	}
	parsed, err := strconv.Atoi(trimmed)
	if err != nil || parsed < 0 {
		return 0, fmt.Errorf("must be a non-negative integer")
	}
	return parsed, nil
}

func boolOrDefault(value string, fallback bool) (bool, error) {
	trimmed := strings.TrimSpace(value)
	if trimmed == "" {
		return fallback, nil
	}
	parsed, err := strconv.ParseBool(trimmed)
	if err != nil {
		return false, fmt.Errorf("must be true or false")
	}
	return parsed, nil
}
