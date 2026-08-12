package community

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/bwmarrin/discordgo"
	"go.opentelemetry.io/contrib/instrumentation/net/http/otelhttp"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/trace"
)

const genericFailureReply = "there was an error generating your reply"

const (
	transportDiscord = "discord"
	transportHTTP    = "http"
	transportMCP     = "mcp"
)

// Agent owns the Sirens Echo Discord session and its outbound boundaries.
type Agent struct {
	cfg               Config
	session           *discordgo.Session
	tools             *MCPProvider
	completions       CompletionClient
	systemPrompt      string
	telemetry         *Telemetry
	readinessClient   *http.Client
	readinessEndpoint string
	readinessRoute    string
	readinessTimeout  time.Duration
	slots             chan struct{}
	seen              *seenMessages
	scope             *channelScope
	access            *AccessPolicy
	limiter           *rateLimiter
	// lookups bounds Discord REST calls forced during gate evaluation, so an
	// unscoped channel cannot make the process call the API per message.
	lookups *rateLimiter
}

// NewAgent builds the independently deployable Sirens Echo runtime.
func NewAgent(cfg Config, telemetry *Telemetry) (*Agent, error) {
	readinessEndpoint, err := agentProxyReadinessEndpoint(
		cfg.AgentProxyURL,
		cfg.AgentProxyModel,
	)
	if err != nil {
		return nil, err
	}
	localSkillpack, err := LoadSkillpack(cfg.Definition.LocalSkillRoots)
	if err != nil {
		return nil, err
	}
	composed := ""
	if cfg.Definition.Composed {
		composed, err = LoadBundle(cfg.BundlePath)
		if err != nil {
			return nil, err
		}
	}
	systemPrompt := BuildSystemPrompt(cfg.Definition, cfg.Principal, composed, localSkillpack)
	if err := ValidateSystemPrompt(cfg.Definition, cfg.Principal, systemPrompt); err != nil {
		return nil, err
	}
	var session *discordgo.Session
	if cfg.DiscordEnabled {
		session, err = discordgo.New("Bot " + cfg.DiscordToken)
		if err != nil {
			return nil, fmt.Errorf("discord session: %w", err)
		}
		session.Identify.Intents = discordgo.IntentsGuilds |
			discordgo.IntentsGuildMessages |
			discordgo.IntentsMessageContent
		if cfg.DiscordDMEnabled {
			session.Identify.Intents |= discordgo.IntentsDirectMessages
		}
	}
	accessPolicy, err := resolveAccessPolicy(cfg)
	if err != nil {
		return nil, err
	}
	telemetry = telemetryOrNoop(telemetry)
	httpClient := &http.Client{
		Timeout: cfg.RequestTimeout,
		Transport: otelhttp.NewTransport(
			http.DefaultTransport,
			otelhttp.WithTracerProvider(telemetry.traceProvider),
			otelhttp.WithPropagators(telemetry.propagator),
		),
	}
	roster, err := loadRoster(cfg)
	if err != nil {
		return nil, err
	}
	tools := &MCPProvider{
		Servers:    roster,
		HTTPClient: httpClient,
	}
	agent := &Agent{
		cfg:     cfg,
		session: session,
		tools:   tools,
		completions: ProxyClient{
			BaseURL:       cfg.AgentProxyURL,
			Model:         cfg.AgentProxyModel,
			AuditRole:     cfg.Definition.AuditRole,
			Attribution:   cfg.Definition.Identity,
			ResponseStyle: cfg.Definition.ResponseStyle,
			Harness:       deploymentHarness(cfg),
			HTTPClient:    httpClient,
			Tools:         tools,
			Telemetry:     telemetry,
		},
		systemPrompt:      systemPrompt,
		telemetry:         telemetry,
		readinessClient:   newReadinessHTTPClient(defaultReadinessTimeout),
		readinessEndpoint: readinessEndpoint,
		readinessRoute:    cfg.AgentProxyModel,
		readinessTimeout:  defaultReadinessTimeout,
		slots:             make(chan struct{}, 1),
		seen:              newSeenMessages(1024),
		scope:             newChannelScope(256),
		access:            accessPolicy,
	}
	agent.ensureRuntimeDefaults()
	if session != nil {
		session.AddHandler(agent.onReady)
		session.AddHandler(agent.onMessage)
	}
	return agent, nil
}

// defaultLookupPolicy bounds Discord REST calls made while evaluating gates.
// See docs/sirens-echo-admission.md.
var defaultLookupPolicy = RateLimitPolicy{
	PerContext: RateLimit{Burst: 30, Every: time.Second},
	Global:     RateLimit{Burst: 60, Every: 500 * time.Millisecond},
}

// ensureRuntimeDefaults fills fields a hand-constructed Agent leaves zero. A
// zero timeout expires on creation and a nil limiter panics.
func (a *Agent) ensureRuntimeDefaults() {
	if a.cfg.RequestTimeout <= 0 {
		a.cfg.RequestTimeout = defaultRequestTimeout
	}
	if a.cfg.QueueTimeout <= 0 {
		a.cfg.QueueTimeout = defaultQueueTimeout
	}
	if a.limiter == nil {
		a.limiter = newRateLimiter(a.cfg.RateLimit, defaultRateLimiterCapacity)
	}
	if a.lookups == nil {
		a.lookups = newRateLimiter(defaultLookupPolicy, 512)
	}
	if a.scope == nil {
		a.scope = newChannelScope(256)
	}
	if a.seen == nil {
		a.seen = newSeenMessages(1024)
	}
	if a.access == nil {
		a.access = synthesizeAccessPolicy(a.cfg)
	}
}

// resolveAccessPolicy prefers the deployment's tracked allowlist file and falls
// back to the equivalent policy built from the legacy environment variables.
func resolveAccessPolicy(cfg Config) (*AccessPolicy, error) {
	if cfg.AccessPolicyPath == "" {
		return synthesizeAccessPolicy(cfg), nil
	}
	return LoadAccessPolicy(cfg.AccessPolicyPath)
}

// deploymentHarness attributes model calls to the ingress this deployment
// actually has, rather than asserting Discord for every profile.
func deploymentHarness(cfg Config) string {
	if cfg.DiscordEnabled {
		return transportDiscord
	}
	return transportHTTP
}

// loadRoster reads the deployment-owned roster and checks the definition's
// issue tracker against it, the one place both are known.
func loadRoster(cfg Config) ([]MCPServerDefinition, error) {
	var roster []MCPServerDefinition
	if cfg.MCPRosterPath != "" {
		loaded, err := LoadMCPRoster(cfg.MCPRosterPath)
		if err != nil {
			return nil, err
		}
		roster = loaded
	}
	tracker := cfg.Definition.IssueTracker
	if tracker == "" {
		return roster, nil
	}
	for _, server := range roster {
		if server.Name == tracker {
			return roster, nil
		}
	}
	return nil, fmt.Errorf("issue_tracker %q names no server in the MCP roster", tracker)
}

// Run opens the Gateway session and blocks until shutdown.
func (a *Agent) Run(ctx context.Context) error {
	if a.session != nil {
		if err := a.session.Open(); err != nil {
			return fmt.Errorf("Discord open: %w", err)
		}
		defer a.session.Close()
	}
	if a.tools != nil {
		// Supervised MCP connections outlive every turn, so shutdown is the only
		// thing that closes them and stops any stdio child.
		defer func() { _ = a.tools.Close() }()
	}
	httpServer := &http.Server{
		Addr:              a.cfg.HTTPListenAddr,
		Handler:           a.HTTPHandler(),
		ReadHeaderTimeout: 5 * time.Second,
		IdleTimeout:       30 * time.Second,
	}
	httpErr := make(chan error, 1)
	go func() {
		httpErr <- httpServer.ListenAndServe()
	}()
	select {
	case <-ctx.Done():
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		if err := httpServer.Shutdown(shutdownCtx); err != nil {
			return fmt.Errorf("HTTP shutdown: %w", err)
		}
		return nil
	case err := <-httpErr:
		if errors.Is(err, http.ErrServerClosed) {
			return nil
		}
		return fmt.Errorf("HTTP serve: %w", err)
	}
}

func (a *Agent) onReady(_ *discordgo.Session, ready *discordgo.Ready) {
	a.telemetry.Info(
		context.Background(),
		"discord.ready",
		slog.String("identity", a.cfg.Definition.Identity),
		slog.String("discord_user", ready.User.Username),
		slog.String("channel", a.cfg.Definition.Channel),
		slog.String("audit_role", a.cfg.Definition.AuditRole),
	)
}

// summonContext is the origin of one summon, and its key is the admission
// identity shared by everything from that guild or direct-message channel.
type summonContext struct {
	Kind      string
	GuildID   string
	ChannelID string
}

const (
	contextKindGuild = "guild"
	contextKindDM    = "dm"
)

// Key identifies the context for admission control. One guild shares one key,
// so it cannot consume every other guild's budget.
func (c summonContext) Key() string {
	if c.Kind == contextKindDM {
		return contextKindDM + ":" + c.ChannelID
	}
	return contextKindGuild + ":" + c.GuildID
}

func (a *Agent) onMessage(session *discordgo.Session, event *discordgo.MessageCreate) {
	message := event.Message
	if !eligibleMessage(session, message) {
		return
	}
	origin := summonContextFor(message)
	// The whole allowlist decides from the payload already in memory, so a
	// guild, member, or channel outside it costs nothing.
	decision := a.access.Evaluate(origin, message.Author.ID, memberRoles(message), nil)
	if !decision.allowed() && decision.Reason != accessNeedsThreadRef {
		a.telemetry.RecordAccess(context.Background(), string(decision.Reason))
		return
	}
	summoned, referenceLookup := summonedLocally(session, message)
	if !summoned && !referenceLookup {
		return
	}
	threadLookup := false
	if decision.Reason == accessNeedsThreadRef {
		cached, known := a.scope.Get(origin.ChannelID)
		if known && !cached {
			a.telemetry.RecordAccess(context.Background(), string(accessDeniedChannel))
			return
		}
		threadLookup = !known
	}
	if threadLookup || (!summoned && referenceLookup) {
		if a.lookups.Admit(admissionRequest{ContextKey: origin.Key()}).Outcome.denied() {
			a.telemetry.RecordAdmission(context.Background(), string(admissionContext), "lookup")
			return
		}
	}
	if threadLookup && !a.resolveScope(session, origin, decision.Guild) {
		a.telemetry.RecordAccess(context.Background(), string(accessDeniedChannel))
		return
	}
	if !summoned && !summonedByReference(session, message) {
		return
	}
	if !a.seen.Add(message.ID) {
		return
	}
	a.telemetry.RecordAccess(context.Background(), string(accessAllowed))
	go a.handleMessage(session, message, origin, decision.Guild.Overrides())
}

// eligibleMessage rejects the payloads that can never be a member summon.
func eligibleMessage(session *discordgo.Session, message *discordgo.Message) bool {
	return message != nil &&
		message.Author != nil &&
		!message.Author.Bot &&
		session.State != nil &&
		session.State.User != nil &&
		message.Author.ID != session.State.User.ID &&
		message.ChannelID != ""
}

// memberRoles returns the author's guild roles, which arrive on the Gateway
// payload. A role grant therefore costs no Discord API call.
func memberRoles(message *discordgo.Message) []string {
	if message.Member == nil {
		return nil
	}
	return message.Member.Roles
}

func summonContextFor(message *discordgo.Message) summonContext {
	if message.GuildID == "" {
		return summonContext{Kind: contextKindDM, ChannelID: message.ChannelID}
	}
	return summonContext{
		Kind:      contextKindGuild,
		GuildID:   message.GuildID,
		ChannelID: message.ChannelID,
	}
}

// resolveScope answers an unknown channel by looking it up and caches both
// outcomes, so a busy unscoped channel does not repeat the lookup per message.
func (a *Agent) resolveScope(
	session *discordgo.Session,
	origin summonContext,
	guild *GuildAccess,
) bool {
	channel := resolveChannel(session, origin.ChannelID)
	if channel == nil {
		// Not cached: the failure may be transient, and caching would deafen a
		// real channel until restart.
		return false
	}
	allowed := channel.IsThread() && guild.PermitsChannel(channel.ParentID)
	a.scope.Set(origin.ChannelID, allowed)
	return allowed
}

func resolveChannel(session *discordgo.Session, channelID string) *discordgo.Channel {
	if session.State != nil {
		if channel, err := session.State.Channel(channelID); err == nil && channel != nil {
			return channel
		}
	}
	channel, err := session.Channel(channelID)
	if err != nil {
		return nil
	}
	return channel
}

// summonedLocally decides summoning from the Gateway payload, and reports
// whether an unresolved reply reference could still make it a summon.
func summonedLocally(
	session *discordgo.Session,
	message *discordgo.Message,
) (summoned bool, referenceLookup bool) {
	// A direct message is addressed to this service by definition. See
	// docs/sirens-echo-contexts.md for what that costs.
	if message.GuildID == "" {
		return true, false
	}
	botID := session.State.User.ID
	for _, mention := range message.Mentions {
		if mention.ID == botID {
			return true, false
		}
	}
	if message.ReferencedMessage != nil && message.ReferencedMessage.Author != nil {
		return message.ReferencedMessage.Author.ID == botID, false
	}
	if message.MessageReference == nil || message.MessageReference.MessageID == "" {
		return false, false
	}
	return false, true
}

func summonedByReference(session *discordgo.Session, message *discordgo.Message) bool {
	referenced, err := session.ChannelMessage(
		message.ChannelID,
		message.MessageReference.MessageID,
	)
	return err == nil &&
		referenced != nil &&
		referenced.Author != nil &&
		referenced.Author.ID == session.State.User.ID
}

func (a *Agent) handleMessage(
	session *discordgo.Session,
	message *discordgo.Message,
	origin summonContext,
	override *RateLimitOverride,
) {
	// A panic in one turn must not take down a deployment serving other
	// guilds, so the handler contains its own failure.
	defer func() {
		if recovered := recover(); recovered != nil {
			a.telemetry.RecordFailure(context.Background(), "panic")
			a.telemetry.Error(
				context.Background(),
				"discord.turn.panicked",
				slog.String("error_type", "turn_panicked"),
			)
		}
	}()

	decision := a.limiter.Admit(admissionRequest{
		UserKey:    message.Author.ID,
		ContextKey: origin.Key(),
		Queued:     true,
		Override:   override,
	})
	if decision.Outcome.denied() {
		a.onDenied(session, message, origin, decision)
		return
	}
	defer a.limiter.Release()
	a.telemetry.RecordAdmission(context.Background(), string(admissionAccepted), transportDiscord)

	receiveCtx, receiveSpan := a.telemetry.StartSpan(
		context.Background(),
		"discord.receive",
		discordMessageSpanAttributes(
			"process",
			message.GuildID,
			message.ChannelID,
			message.ID,
		)...,
	)
	defer receiveSpan.End()
	turn := &discordMessageTurn{
		session: session,
		message: message,
		limit:   a.cfg.Definition.MaxContextMessages,
	}
	if err := a.runSerialized(receiveCtx, turn, origin.Key()); err != nil {
		a.telemetry.MarkSpanError(receiveSpan, exceptionTurnFailed)
		a.telemetry.Error(receiveCtx, "discord.turn.failed", slog.String("error_type", "turn_failed"))
	}
}

// onDenied records a refused summon and tells the member at most once per
// notify window, so a flooder gains no amplifier.
func (a *Agent) onDenied(
	session *discordgo.Session,
	message *discordgo.Message,
	origin summonContext,
	decision admissionDecision,
) {
	ctx := context.Background()
	a.telemetry.RecordAdmission(ctx, string(decision.Outcome), transportDiscord)
	a.telemetry.Info(
		ctx,
		"turn.input.denied",
		slog.String("transport", transportDiscord),
		slog.String("outcome", string(decision.Outcome)),
		slog.String("context_kind", origin.Kind),
		slog.Bool("notified", decision.Notify),
	)
	if !decision.Notify {
		return
	}
	turn := &discordMessageTurn{session: session, message: message}
	if err := turn.Reply(ctx, cooldownNotice(decision.RetryAfter)); err != nil {
		a.telemetry.RecordFailure(ctx, "reply")
	}
}

func cooldownNotice(retryAfter time.Duration) string {
	if retryAfter < time.Second {
		return "Rate limit reached. Try again shortly."
	}
	return fmt.Sprintf(
		"Rate limit reached. Try again in about %s.",
		retryAfter.Round(time.Second),
	)
}

// runSerialized waits for the execution slot, then runs the turn. The request
// budget starts after admission, not on arrival.
func (a *Agent) runSerialized(ctx context.Context, turn turnIO, contextKey string) error {
	queueCtx, cancelQueue := context.WithTimeout(ctx, a.cfg.QueueTimeout)
	defer cancelQueue()
	select {
	case a.slots <- struct{}{}:
	case <-queueCtx.Done():
		a.telemetry.RecordAdmission(ctx, string(admissionQueue), turn.Transport())
		a.noticeQueueTimeout(ctx, turn, contextKey)
		return fmt.Errorf("turn waited longer than %s for the execution slot", a.cfg.QueueTimeout)
	}
	defer func() { <-a.slots }()

	turnCtx, cancel := context.WithTimeout(ctx, a.cfg.RequestTimeout)
	defer cancel()
	// Typing starts when the turn runs. Started at queue time it would expire
	// before the reply.
	if notifier, ok := turn.(typingNotifier); ok {
		stopTyping := startTyping(turnCtx, notifier)
		defer stopTyping()
	}
	return a.runTurn(turnCtx, turn)
}

// noticeQueueTimeout tells the caller its turn gave up waiting. Returning
// silently left a queued member with no reply at all.
func (a *Agent) noticeQueueTimeout(ctx context.Context, turn turnIO, contextKey string) {
	// A Discord reply lands in a shared channel, so it shares the throttle the
	// pending-cap denial uses. A synchronous caller always learns why it ended.
	if turn.Transport() == transportDiscord &&
		!a.limiter.notifyQueueTimeout(contextKey) {
		return
	}
	if err := turn.Reply(ctx, queueTimeoutNotice); err != nil {
		a.telemetry.RecordFailure(ctx, "reply")
	}
}

// queueTimeoutNotice matches the cooldown notice's impersonal shape, since the
// neutral style rules bind every member-facing string.
const queueTimeoutNotice = "Busy with another turn. Try again shortly."

// typingNotifier is implemented by transports that can show progress.
type typingNotifier interface {
	Typing() error
}

// startTyping holds the indicator for the turn. Discord expires it after
// roughly ten seconds, so a long model call needs it refreshed.
func startTyping(ctx context.Context, notifier typingNotifier) func() {
	_ = notifier.Typing()
	done := make(chan struct{})
	go func() {
		ticker := time.NewTicker(8 * time.Second)
		defer ticker.Stop()
		for {
			select {
			case <-done:
				return
			case <-ctx.Done():
				return
			case <-ticker.C:
				_ = notifier.Typing()
			}
		}
	}()
	return func() { close(done) }
}

type turnIO interface {
	RequestID() string
	Transport() string
	Current() TranscriptEntry
	History(ctx context.Context) ([]TranscriptEntry, error)
	Reply(ctx context.Context, content string) error
}

func (a *Agent) runTurn(ctx context.Context, turn turnIO) (turnErr error) {
	started := time.Now()
	turnCtx, turnSpan := a.telemetry.StartSpan(
		ctx,
		"community.turn",
		attribute.String("sirens_echo.transport", turn.Transport()),
		attribute.String("sirens_echo.request_id", turn.RequestID()),
		attribute.String("agent.role", a.cfg.Definition.AuditRole),
	)
	if turn.Transport() == transportDiscord {
		turnSpan.SetAttributes(attribute.String("discord.channel", a.cfg.Definition.Channel))
	}
	outcome := "ok"
	defer func() {
		if turnErr != nil {
			outcome = "error"
			a.telemetry.MarkSpanError(turnSpan, exceptionTurnFailed)
		}
		a.telemetry.RecordTurn(turnCtx, outcome, time.Since(started))
		turnSpan.End()
	}()

	receiveCtx, receiveSpan := a.telemetry.StartSpan(turnCtx, "community.input")
	current := turn.Current()
	a.telemetry.Info(
		receiveCtx,
		"turn.input.accepted",
		slog.String("transport", turn.Transport()),
		slog.String("request_id", turn.RequestID()),
		slog.Int("input_bytes", len(current.Content)),
	)
	receiveSpan.SetAttributes(attribute.Int("input.bytes", len(current.Content)))
	receiveSpan.End()

	historyCtx, historySpan := a.telemetry.StartSpan(turnCtx, "community.history")
	history, err := turn.History(historyCtx)
	if err != nil {
		a.telemetry.MarkSpanError(historySpan, exceptionHistoryFailed)
		historySpan.End()
		return a.failTurn(turnCtx, turn, "history", err)
	}
	historySpan.SetAttributes(attribute.Int("history.count", len(history)))
	historySpan.End()

	contextCtx, contextSpan := a.telemetry.StartSpan(turnCtx, "context.assemble")
	userPrompt := BuildUserPrompt(history, current)
	systemPrompt := a.systemPrompt
	a.telemetry.Info(
		contextCtx,
		"context.rendered",
		slog.Int("history_count", len(history)),
		slog.Int("system_prompt_bytes", len(systemPrompt)),
		slog.Int("user_prompt_bytes", len(userPrompt)),
	)
	contextSpan.SetAttributes(
		attribute.Int("history.count", len(history)),
		attribute.Int("prompt.system.bytes", len(systemPrompt)),
		attribute.Int("prompt.user.bytes", len(userPrompt)),
	)
	contextSpan.End()

	result, err := a.completions.Complete(
		turnCtx,
		systemPrompt,
		userPrompt,
		turn.RequestID(),
	)
	if err != nil {
		return a.failTurn(turnCtx, turn, "model", err)
	}

	_, validateSpan := a.telemetry.StartSpan(turnCtx, "response.validate")
	reply, err := ParseReply(result.Content)
	if err == nil {
		err = ValidateGrounding(reply, systemPrompt+"\n"+userPrompt, result.ToolCalls...)
	}
	if err == nil {
		err = ValidateResponseStyle(a.cfg.Definition.ResponseStyle, reply)
	}
	if err != nil {
		a.telemetry.MarkSpanError(validateSpan, exceptionResponseValidationFailed)
		validateSpan.End()
		return a.failTurn(turnCtx, turn, "validation", err)
	}
	validateSpan.End()

	if err := a.sendReply(turnCtx, turn, reply); err != nil {
		return err
	}
	return nil
}

func (a *Agent) failTurn(
	ctx context.Context,
	turn turnIO,
	stage string,
	cause error,
) error {
	a.telemetry.RecordFailure(ctx, stage)
	a.telemetry.Error(
		ctx,
		"turn.stage.failed",
		slog.String("stage", stage),
		slog.String("error_type", stage+"_failed"),
	)
	replyErr := a.sendReply(ctx, turn, genericFailureReply)
	return errors.Join(cause, replyErr)
}

func (a *Agent) sendReply(ctx context.Context, turn turnIO, content string) error {
	replyCtx, replySpan := a.telemetry.StartSpan(ctx, "community.reply")
	a.telemetry.Info(
		replyCtx,
		"turn.reply.ready",
		slog.String("transport", turn.Transport()),
		slog.Int("reply_bytes", len(content)),
	)
	var err error
	if turn.Transport() == transportDiscord {
		discordCtx, discordSpan := a.telemetry.StartSpan(replyCtx, "discord.reply")
		err = turn.Reply(discordCtx, content)
		if err != nil {
			a.telemetry.MarkSpanError(discordSpan, exceptionDiscordReplyFailed)
		}
		discordSpan.End()
	} else {
		err = turn.Reply(replyCtx, content)
	}
	if err != nil {
		a.telemetry.RecordFailure(replyCtx, "reply")
		a.telemetry.MarkSpanError(replySpan, exceptionReplyFailed)
	}
	replySpan.End()
	return err
}

type discordMessageTurn struct {
	session *discordgo.Session
	message *discordgo.Message
	limit   int
}

func (t *discordMessageTurn) RequestID() string {
	return t.message.ID
}

func (t *discordMessageTurn) Transport() string { return transportDiscord }

func (t *discordMessageTurn) Current() TranscriptEntry {
	return TranscriptEntry{
		Author:  displayName(t.message),
		Content: t.message.ContentWithMentionsReplaced(),
	}
}

func (t *discordMessageTurn) History(_ context.Context) ([]TranscriptEntry, error) {
	messages, err := t.session.ChannelMessages(
		t.message.ChannelID,
		t.limit,
		t.message.ID,
		"",
		"",
	)
	if err != nil {
		return nil, err
	}
	history := make([]TranscriptEntry, 0, len(messages))
	for index := len(messages) - 1; index >= 0; index-- {
		message := messages[index]
		if message.Author == nil {
			continue
		}
		history = append(history, TranscriptEntry{
			Author:  displayName(message),
			Content: message.ContentWithMentionsReplaced(),
		})
	}
	return history, nil
}

// Typing shows the Discord indicator for this turn's channel.
func (t *discordMessageTurn) Typing() error {
	return t.session.ChannelTyping(t.message.ChannelID)
}

func (t *discordMessageTurn) Reply(ctx context.Context, content string) error {
	span := trace.SpanFromContext(ctx)
	span.SetAttributes(discordMessageSpanAttributes(
		"send",
		t.message.GuildID,
		t.message.ChannelID,
		"",
	)...)
	reply, err := t.session.ChannelMessageSendComplex(t.message.ChannelID, &discordgo.MessageSend{
		Content:   truncateRunes(content, 1990),
		Reference: t.message.SoftReference(),
		AllowedMentions: &discordgo.MessageAllowedMentions{
			Parse:       []discordgo.AllowedMentionType{},
			RepliedUser: false,
		},
	})
	if reply != nil && reply.ID != "" {
		span.SetAttributes(attribute.String("messaging.message.id", reply.ID))
	}
	return err
}

func discordMessageSpanAttributes(
	operation, guildID, channelID, messageID string,
) []attribute.KeyValue {
	attributes := []attribute.KeyValue{
		attribute.String("messaging.system", "discord"),
		attribute.String("messaging.operation.name", operation),
		attribute.String("messaging.operation.type", operation),
		attribute.String("discord.guild.id", guildID),
		attribute.String("discord.channel.id", channelID),
	}
	if messageID != "" {
		attributes = append(
			attributes,
			attribute.String("messaging.message.id", messageID),
		)
	}
	return attributes
}

func displayName(message *discordgo.Message) string {
	if message.Member != nil && strings.TrimSpace(message.Member.Nick) != "" {
		return message.Member.Nick
	}
	if message.Author != nil {
		if strings.TrimSpace(message.Author.GlobalName) != "" {
			return message.Author.GlobalName
		}
		return message.Author.Username
	}
	return "member"
}

func truncateRunes(value string, limit int) string {
	runes := []rune(value)
	if len(runes) <= limit {
		return value
	}
	if limit <= 1 {
		return string(runes[:limit])
	}
	return string(runes[:limit-1]) + "…"
}

type channelScope struct {
	mu       sync.Mutex
	capacity int
	order    []string
	values   map[string]bool
}

func newChannelScope(capacity int) *channelScope {
	return &channelScope{
		capacity: capacity,
		order:    make([]string, 0, capacity),
		values:   make(map[string]bool, capacity),
	}
}

func (c *channelScope) Get(id string) (bool, bool) {
	c.mu.Lock()
	defer c.mu.Unlock()
	allowed, known := c.values[id]
	return allowed, known
}

func (c *channelScope) Set(id string, allowed bool) {
	c.mu.Lock()
	defer c.mu.Unlock()
	if _, exists := c.values[id]; exists {
		return
	}
	c.values[id] = allowed
	c.order = append(c.order, id)
	if len(c.order) > c.capacity {
		oldest := c.order[0]
		c.order = c.order[1:]
		delete(c.values, oldest)
	}
}

type seenMessages struct {
	mu       sync.Mutex
	capacity int
	order    []string
	values   map[string]struct{}
}

func newSeenMessages(capacity int) *seenMessages {
	return &seenMessages{
		capacity: capacity,
		order:    make([]string, 0, capacity),
		values:   make(map[string]struct{}, capacity),
	}
}

func (s *seenMessages) Add(id string) bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	if _, exists := s.values[id]; exists {
		return false
	}
	s.values[id] = struct{}{}
	s.order = append(s.order, id)
	if len(s.order) > s.capacity {
		oldest := s.order[0]
		s.order = s.order[1:]
		delete(s.values, oldest)
	}
	return true
}
