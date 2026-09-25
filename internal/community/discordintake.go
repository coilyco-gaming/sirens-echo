package community

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"strings"
	"sync/atomic"
	"time"

	"github.com/bwmarrin/discordgo"
)

// A gateway session that only writes what it receives to the Discord event
// queue. See docs/sirens-echo-jobs.md.

// discordOfferer turns gateway events into queue offers. The intake and a
// queue-mode worker both register it, so every session derives the same keys.
type discordOfferer struct {
	queue     DiscordEventQueue
	telemetry *Telemetry
	now       func() time.Time
}

func (o *discordOfferer) onMessage(session *discordgo.Session, event *discordgo.MessageCreate) {
	if event == nil || event.Message == nil || event.Author == nil {
		return
	}
	// This service's own posts are never summons, and are most of the volume in
	// a busy thread.
	if session != nil && session.State != nil && session.State.User != nil &&
		event.Author.ID == session.State.User.ID {
		return
	}
	o.offer(discordEventMessage, "discord:msg:"+event.ID, event.Message)
}

func (o *discordOfferer) onMessageEdit(session *discordgo.Session, event *discordgo.MessageUpdate) {
	// Filtered here, because a link preview arrives as an edit of every message
	// carrying a link, and none of those can summon.
	if !editSummons(session, event) {
		return
	}
	edited := event.EditedTimestamp.UTC().Format(time.RFC3339Nano)
	o.offer(discordEventEdit, "discord:edit:"+event.ID+":"+edited, event.Message)
}

func (o *discordOfferer) onInteraction(_ *discordgo.Session, event *discordgo.InteractionCreate) {
	if event == nil || event.Interaction == nil || event.Type != discordgo.InteractionApplicationCommand {
		return
	}
	o.offer(discordEventInteraction, "discord:interaction:"+event.ID, event.Interaction)
}

func (o *discordOfferer) offer(kind, key string, payload any) {
	ctx := context.Background()
	telemetry := telemetryOrNoop(o.telemetry)
	raw, err := json.Marshal(payload)
	if err != nil {
		telemetry.Error(ctx, "discord.intake.encode_failed",
			slog.String("kind", kind), slog.String("error", err.Error()))
		return
	}
	now := time.Now().UTC()
	if o.now != nil {
		now = o.now()
	}
	inserted, err := o.queue.Offer(ctx, DiscordEvent{Key: key, Kind: kind, Payload: raw, ReceivedAt: now})
	if err != nil {
		telemetry.Error(ctx, "discord.intake.offer_failed",
			slog.String("kind", kind), slog.String("error", err.Error()))
		return
	}
	outcome := "duplicate"
	if inserted {
		outcome = "offered"
	}
	telemetry.RecordDiscordEvent(ctx, kind, outcome)
}

// discordIntents is the one intent set every session of a deployment opens
// with, so an intake cannot see less than the worker it replaced.
func discordIntents(cfg Config) discordgo.Intent {
	intents := discordgo.IntentsGuilds | discordgo.IntentsGuildMessages | discordgo.IntentsMessageContent
	if cfg.DiscordDMEnabled {
		intents |= discordgo.IntentsDirectMessages
	}
	return intents
}

// LoadIntakeConfig reads what the intake needs and nothing else, since it holds
// no definition, model route, or access policy.
func LoadIntakeConfig() (Config, error) {
	cfg := Config{
		DiscordToken:   strings.TrimSpace(os.Getenv("DISCORD_TOKEN")),
		JobStoreDSN:    strings.TrimSpace(os.Getenv("SIRENS_ECHO_JOB_STORE_DSN")),
		OTLPEndpoint:   valueOrDefault(os.Getenv("OTEL_EXPORTER_OTLP_ENDPOINT"), DefaultOTLPEndpoint),
		HTTPListenAddr: valueOrDefault(os.Getenv("SIRENS_ECHO_HTTP_ADDR"), defaultHTTPListenAddr),
		InstanceName:   valueOrDefault(os.Getenv("SIRENS_ECHO_INSTANCE"), defaultInstanceName) + "-intake",
	}
	if err := applyFeatureFlags(&cfg, os.Getenv); err != nil {
		return Config{}, err
	}
	missing := make([]string, 0, 2)
	if cfg.DiscordToken == "" {
		missing = append(missing, "DISCORD_TOKEN")
	}
	if cfg.JobStoreDSN == "" {
		missing = append(missing, "SIRENS_ECHO_JOB_STORE_DSN")
	}
	if len(missing) > 0 {
		return Config{}, fmt.Errorf("missing required env: %v", missing)
	}
	return cfg, nil
}

// Intake holds one gateway session and writes every event it receives to the
// queue. Several run at once, and the queue's key makes that safe.
type Intake struct {
	cfg       Config
	session   *discordgo.Session
	telemetry *Telemetry
	queue     *PostgresDiscordEventQueue
	// ready is set by READY and RESUMED and cleared by a disconnect, so a
	// rolling update never counts a reconnecting session as serving.
	ready atomic.Bool
}

// NewIntake builds the session and its handlers. Nothing connects until Run.
func NewIntake(cfg Config, telemetry *Telemetry) (*Intake, error) {
	session, err := discordgo.New("Bot " + cfg.DiscordToken)
	if err != nil {
		return nil, fmt.Errorf("discord session: %w", err)
	}
	session.Identify.Intents = discordIntents(cfg)
	return &Intake{cfg: cfg, session: session, telemetry: telemetryOrNoop(telemetry)}, nil
}

// Run connects the queue and the gateway, serves the probes, and blocks until
// ctx ends. Commands are never registered here, which is the worker's job.
func (i *Intake) Run(ctx context.Context) error {
	queue, err := OpenPostgresDiscordEventQueue(ctx, i.cfg.JobStoreDSN)
	if err != nil {
		return err
	}
	defer queue.Close()
	i.queue = queue
	offerer := &discordOfferer{queue: queue, telemetry: i.telemetry}
	i.session.AddHandler(offerer.onMessage)
	i.session.AddHandler(offerer.onMessageEdit)
	if i.cfg.DiscordCommandsEnabled {
		i.session.AddHandler(offerer.onInteraction)
	}
	i.session.AddHandler(func(_ *discordgo.Session, ready *discordgo.Ready) {
		i.ready.Store(true)
		i.telemetry.Info(ctx, "discord.intake.ready", slog.String("session_id", ready.SessionID))
	})
	i.session.AddHandler(func(*discordgo.Session, *discordgo.Resumed) { i.ready.Store(true) })
	i.session.AddHandler(func(*discordgo.Session, *discordgo.Disconnect) {
		i.ready.Store(false)
		i.telemetry.Info(ctx, "discord.intake.disconnected")
	})
	if err := i.session.Open(); err != nil {
		return fmt.Errorf("Discord open: %w", err)
	}
	defer i.session.Close()
	server := &http.Server{
		Addr:              i.cfg.HTTPListenAddr,
		Handler:           i.HTTPHandler(),
		ReadHeaderTimeout: 5 * time.Second,
		IdleTimeout:       30 * time.Second,
	}
	served := make(chan error, 1)
	go func() { served <- server.ListenAndServe() }()
	select {
	case <-ctx.Done():
		// Not ready first, so the Service stops routing before the socket closes.
		i.ready.Store(false)
		shutdown, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		return server.Shutdown(shutdown)
	case err := <-served:
		if errors.Is(err, http.ErrServerClosed) {
			return nil
		}
		return fmt.Errorf("HTTP serve: %w", err)
	}
}

// HTTPHandler serves liveness, and readiness that holds until the gateway has
// identified and the queue answers.
func (i *Intake) HTTPHandler() http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc(healthzPath, func(writer http.ResponseWriter, _ *http.Request) {
		writeJSON(writer, http.StatusOK, map[string]bool{"ok": true})
	})
	mux.HandleFunc(readyzPath, func(writer http.ResponseWriter, request *http.Request) {
		if reason := i.notReady(request.Context()); reason != "" {
			writeJSON(writer, http.StatusServiceUnavailable, map[string]string{"status": reason})
			return
		}
		writeJSON(writer, http.StatusOK, map[string]string{"status": "ready"})
	})
	return mux
}

func (i *Intake) notReady(ctx context.Context) string {
	if !i.ready.Load() {
		return "gateway_not_ready"
	}
	if i.queue == nil {
		return "queue_not_open"
	}
	ctx, cancel := context.WithTimeout(ctx, 2*time.Second)
	defer cancel()
	if err := i.queue.Ping(ctx); err != nil {
		return "queue_unreachable"
	}
	return ""
}
