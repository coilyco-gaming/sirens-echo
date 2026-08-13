package community

import (
	"fmt"
	"io"
	"net/url"
	"os"
	"path/filepath"
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
	defaultBundleDir      = "/app/agent/bundles"
	defaultComposedRole   = "creator"

	ResponseStyleNeutral = "neutral"
	ResponseStyleSocial  = "social"
)

// Admission defaults are sized for a guild the operator does not moderate.
// See docs/sirens-echo-admission.md.

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
	composedRolePattern    = regexp.MustCompile(`^[a-z][a-z0-9-]*$`)
	// channelLabelPattern matches the grounding validator's channel form, so a
	// label cannot introduce a reference the model is rejected for repeating.
	channelLabelPattern = regexp.MustCompile(`^#[A-Za-z_][A-Za-z0-9_-]*$`)
	// rateLimitPattern is "<burst>/<refill interval for one token>".
	rateLimitPattern = regexp.MustCompile(`^([0-9]+)/(.+)$`)
)

// MCP client transports. Deployment picks the servers and their transports.
// Echo validates the shape of an entry, never which server is acceptable.
const (
	MCPTransportStreamable = "streamable"
	MCPTransportSSE        = "sse"
	MCPTransportStdio      = "stdio"
)

// MCPServerDefinition is one resolved roster entry. Deployment owns the roster,
// so this is built by the loader rather than written in a source definition.
type MCPServerDefinition struct {
	Name      string
	Transport string
	URL       string
	Command   string
	Args      []string
	Env       map[string]string
}

// ResolvedTransport defaults to streamable, so an entry written before
// transports were selectable keeps working unchanged.
func (s MCPServerDefinition) ResolvedTransport() string {
	if trimmed := strings.TrimSpace(s.Transport); trimmed != "" {
		return trimmed
	}
	return MCPTransportStreamable
}

// Definition is the source-controlled attribution, route, and policy selection.
type Definition struct {
	Schema             string   `json:"schema" yaml:"schema"`
	Identity           string   `json:"identity" yaml:"identity"`
	AuditRole          string   `json:"audit_role" yaml:"audit_role"`
	ResponseStyle      string   `json:"response_style" yaml:"response_style"`
	Channel            string   `json:"channel" yaml:"channel"`
	MaxContextMessages int      `json:"max_context_messages" yaml:"max_context_messages"`
	LocalSkillRoots    []string `json:"local_skill_roots" yaml:"local_skill_roots"`
	IssueTracker       string   `json:"issue_tracker,omitempty" yaml:"issue_tracker,omitempty"`
	// Composed requires a materialized agent-compose bundle, so a profile that
	// asks for an identity fails startup rather than answering without one.
	Composed bool `json:"composed,omitempty" yaml:"composed,omitempty"`
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
	// LogWriter receives structured logs. Empty means stdout, which is what the
	// service wants. A runner emitting a dataset on stdout selects stderr.
	LogWriter io.Writer
	// Principal is empty until deployment names the trusted account.
	Principal Principal
	// BundlePath is the materialized bundle for the deployment's role, empty
	// when the definition composes nothing.
	BundlePath     string
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
	// MCPRosterPath names the deployment-owned mcpServers file. Empty is a
	// valid no-tool boundary.
	MCPRosterPath string
	// AccessPolicyPath names the deployment's tracked allowlist file. Empty
	// synthesizes the equivalent from the Discord environment variables.
	AccessPolicyPath string
	// DiscordCommandsEnabled registers and serves application commands. Off by
	// default because registering is a write to Discord's API.
	DiscordCommandsEnabled bool
	// JobWorkspaceRoot enables executing jobs. Empty means no execution at all,
	// which is the default posture.
	JobWorkspaceRoot string
	// JobRepository and JobVerb fix what an executing job does, both from
	// closed sets in this repository.
	JobRepository string
	JobVerb       string
	// JobStoreDir is the durable job store's directory. Empty keeps jobs in
	// memory, which loses them on restart. See docs/sirens-echo-jobs.md.
	JobStoreDir string
	// ScratchDir mounts the per-requester scratchpad. Empty offers no scratchpad
	// tools at all. See docs/sirens-echo-scratchpad.md.
	ScratchDir     string
	RequestTimeout time.Duration
	QueueTimeout   time.Duration
	RateLimit      RateLimitPolicy
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
	commandsEnabled, err := boolOrDefault(os.Getenv("SIRENS_ECHO_DISCORD_COMMANDS"), false)
	if err != nil {
		return Config{}, fmt.Errorf("SIRENS_ECHO_DISCORD_COMMANDS: %w", err)
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
		DiscordEnabled:         discordEnabled,
		DiscordToken:           strings.TrimSpace(os.Getenv("DISCORD_TOKEN")),
		DiscordChannelIDs:      splitList(os.Getenv("DISCORD_CHANNEL_ID")),
		DiscordGuildIDs:        splitList(os.Getenv("DISCORD_GUILD_IDS")),
		DiscordDMEnabled:       dmEnabled,
		DiscordCommandsEnabled: commandsEnabled,
		AgentProxyURL:          valueOrDefault(os.Getenv("AGENT_PROXY_URL"), DefaultAgentProxyURL),
		AgentProxyModel:        strings.TrimSpace(os.Getenv("AGENT_PROXY_MODEL")),
		OTLPEndpoint:           valueOrDefault(os.Getenv("OTEL_EXPORTER_OTLP_ENDPOINT"), DefaultOTLPEndpoint),
		HTTPListenAddr:         valueOrDefault(os.Getenv("SIRENS_ECHO_HTTP_ADDR"), defaultHTTPListenAddr),
		MCPRosterPath:          strings.TrimSpace(os.Getenv("SIRENS_ECHO_MCP_ROSTER")),
		AccessPolicyPath:       strings.TrimSpace(os.Getenv("SIRENS_ECHO_ACCESS_POLICY")),
		JobStoreDir:            strings.TrimSpace(os.Getenv("SIRENS_ECHO_JOB_STORE")),
		ScratchDir:             strings.TrimSpace(os.Getenv("SIRENS_ECHO_SCRATCH")),
		RequestTimeout:         requestTimeout,
		QueueTimeout:           queueTimeout,
		RateLimit:              rateLimit,
	}
	if !mcpServerNamePattern.MatchString(cfg.InstanceName) {
		return Config{}, fmt.Errorf("SIRENS_ECHO_INSTANCE must be a lowercase service name")
	}
	if err := validatePrincipal(cfg.Principal); err != nil {
		return Config{}, err
	}
	if cfg.Definition.Composed {
		path, err := resolveBundlePath()
		if err != nil {
			return Config{}, err
		}
		cfg.BundlePath = path
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

// resolveBundlePath selects the baked bundle for the deployment's role. The
// image bakes one per role, so a role flip needs no rebuild.
func resolveBundlePath() (string, error) {
	role := valueOrDefault(strings.TrimSpace(os.Getenv("SIRENS_DEEP_ROLE")), defaultComposedRole)
	if !composedRolePattern.MatchString(role) {
		return "", fmt.Errorf("SIRENS_DEEP_ROLE must be a lowercase role slug, got %q", role)
	}
	dir := valueOrDefault(strings.TrimSpace(os.Getenv("SIRENS_ECHO_BUNDLE_DIR")), defaultBundleDir)
	path := filepath.Join(dir, role)
	if _, err := os.Stat(filepath.Join(path, "manifest.json")); err != nil {
		return "", fmt.Errorf("no composed bundle for role %q under %s: %w", role, dir, err)
	}
	return path, nil
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
	// The roster is deployment-owned, so whether this names a live server is
	// checked where both are known rather than here.
	if definition.IssueTracker != "" &&
		!mcpServerNamePattern.MatchString(definition.IssueTracker) {
		return Definition{}, fmt.Errorf("invalid issue_tracker %q", definition.IssueTracker)
	}
	return definition, nil
}

// validateMCPServer checks that an entry carries the fields its transport needs
// and none belonging to another. It makes no judgement about which server runs.
func validateMCPServer(server MCPServerDefinition) error {
	hasURL := strings.TrimSpace(server.URL) != ""
	hasCommand := strings.TrimSpace(server.Command) != ""
	switch server.ResolvedTransport() {
	case MCPTransportStreamable, MCPTransportSSE:
		// An unset interpolation variable lands here as an empty endpoint.
		if !hasURL {
			return fmt.Errorf("MCP server %q requires a baseUrl", server.Name)
		}
		if !validHTTPURL(server.URL) {
			return fmt.Errorf("MCP server %q has invalid baseUrl", server.Name)
		}
		if hasCommand || len(server.Args) > 0 || len(server.Env) > 0 {
			return fmt.Errorf("MCP server %q takes no command, args, or env", server.Name)
		}
	case MCPTransportStdio:
		if !hasCommand {
			return fmt.Errorf("MCP server %q requires a command", server.Name)
		}
		if hasURL {
			return fmt.Errorf("MCP server %q takes no baseUrl", server.Name)
		}
		for name := range server.Env {
			if !environmentNamePattern.MatchString(name) {
				return fmt.Errorf("MCP server %q has invalid env name %q", server.Name, name)
			}
		}
	default:
		return fmt.Errorf("MCP server %q has unsupported transport %q", server.Name, server.Transport)
	}
	return nil
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
