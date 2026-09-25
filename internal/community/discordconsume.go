package community

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"os"
	"sync"
	"time"

	"github.com/bwmarrin/discordgo"
)

// How a queue-mode worker takes the Discord events an intake wrote. See
// docs/sirens-echo-jobs.md.

// Discord refuses an interaction answered more than three seconds after it was
// sent, so an older one is dropped rather than answered into an error.
const discordInteractionWindow = 3 * time.Second

// discordEventHandlers is what a claimed event is dispatched to, so the loop is
// testable without an Agent.
type discordEventHandlers struct {
	message     func(*discordgo.Message)
	interaction func(*discordgo.InteractionCreate)
}

// openDiscordEventQueue shares the job store's pool, which is the database the
// intake was pointed at.
func openDiscordEventQueue(store JobStore) (DiscordEventQueue, error) {
	postgres, ok := store.(*PostgresJobStore)
	if !ok {
		return nil, errors.New("SIRENS_ECHO_DISCORD_QUEUE needs the Postgres job store")
	}
	return newPostgresDiscordEventQueue(context.Background(), postgres.pool)
}

// consumeDiscordEvents claims and dispatches until ctx ends, sweeping old rows
// on the side so the table stays the size of a day.
func consumeDiscordEvents(
	ctx context.Context,
	queue DiscordEventQueue,
	owner string,
	handlers discordEventHandlers,
	telemetry *Telemetry,
) {
	sweep := time.NewTicker(discordQueueSweepEvery)
	defer sweep.Stop()
	for {
		claimed := drainDiscordEvents(ctx, queue, owner, handlers, telemetry, time.Now)
		if claimed > 0 {
			continue
		}
		select {
		case <-ctx.Done():
			return
		case <-sweep.C:
			swept, err := queue.Sweep(ctx, time.Now().Add(-discordQueueRetention))
			if err != nil {
				telemetry.Error(ctx, "discord.queue.sweep_failed", slog.String("error", err.Error()))
				continue
			}
			telemetry.Info(ctx, "discord.queue.swept", slog.Int("events", swept))
		case <-time.After(discordQueuePoll):
		}
	}
}

// drainDiscordEvents claims one batch and dispatches it, returning how many it
// claimed. A shutdown stops the next claim, never a claimed batch.
func drainDiscordEvents(
	ctx context.Context,
	queue DiscordEventQueue,
	owner string,
	handlers discordEventHandlers,
	telemetry *Telemetry,
	now func() time.Time,
) int {
	if ctx.Err() != nil {
		return 0
	}
	events, err := queue.Claim(ctx, owner, discordQueueBatch)
	if err != nil {
		telemetry.Error(ctx, "discord.queue.claim_failed", slog.String("error", err.Error()))
		return 0
	}
	for _, event := range events {
		dispatchDiscordEvent(ctx, event, handlers, telemetry, now())
	}
	return len(events)
}

func dispatchDiscordEvent(
	ctx context.Context,
	event DiscordEvent,
	handlers discordEventHandlers,
	telemetry *Telemetry,
	now time.Time,
) {
	age := now.Sub(event.ReceivedAt)
	outcome := "dispatched"
	defer func() { telemetry.RecordDiscordEvent(ctx, event.Kind, outcome) }()
	switch event.Kind {
	case discordEventMessage, discordEventEdit:
		if age > discordEventMaxAge {
			outcome = "expired"
			return
		}
		var message discordgo.Message
		if err := json.Unmarshal(event.Payload, &message); err != nil {
			outcome = "undecodable"
			return
		}
		if handlers.message == nil {
			outcome = "unhandled"
			return
		}
		handlers.message(&message)
	case discordEventInteraction:
		if age > discordInteractionWindow {
			outcome = "expired"
			return
		}
		var interaction discordgo.Interaction
		if err := json.Unmarshal(event.Payload, &interaction); err != nil {
			outcome = "undecodable"
			return
		}
		if handlers.interaction == nil {
			outcome = "unhandled"
			return
		}
		handlers.interaction(&discordgo.InteractionCreate{Interaction: &interaction})
	default:
		outcome = "unknown_kind"
	}
}

// discordEventHandlers routes claimed events through the same admission a
// connected session's handlers use.
func (a *Agent) discordEventHandlers() discordEventHandlers {
	handlers := discordEventHandlers{
		message: func(message *discordgo.Message) {
			a.hydrateDiscordState(message.GuildID, message.ChannelID)
			a.admitMessage(a.session, message)
		},
	}
	if a.cfg.DiscordCommandsEnabled {
		handlers.interaction = func(event *discordgo.InteractionCreate) {
			a.hydrateDiscordState(event.GuildID, event.ChannelID)
			a.onInteraction(a.session, event)
		}
	}
	return handlers
}

// startDiscordQueue begins consuming. The owner names this process on a claimed
// row, for diagnosis only.
func (a *Agent) startDiscordQueue(ctx context.Context) {
	owner, _ := os.Hostname()
	go consumeDiscordEvents(ctx, a.events, owner, a.discordEventHandlers(), a.telemetry)
}

// connectDiscordREST stands in for READY on a session that never opens the
// gateway: it learns this account's identity, then runs the ready path.
func (a *Agent) connectDiscordREST() error {
	user, err := a.session.User("@me")
	if err != nil {
		return fmt.Errorf("Discord identity over REST: %w", err)
	}
	a.session.State.User = user
	a.onReady(a.session, &discordgo.Ready{User: user})
	return nil
}

// hydrateDiscordState fills the cache a connected session gets from the
// gateway, one REST read per guild and channel. Only a closed gateway needs it.
func (a *Agent) hydrateDiscordState(guildID, channelID string) {
	if a.cfg.DiscordGateway || a.session == nil || a.session.State == nil {
		return
	}
	state := a.session.State
	if guildID != "" {
		if _, err := state.Guild(guildID); err != nil && a.hydrated.due("guild:"+guildID, time.Now()) {
			if guild, err := a.session.Guild(guildID); err == nil {
				_ = state.GuildAdd(guild)
			}
		}
	}
	if channelID != "" {
		if _, err := state.Channel(channelID); err != nil && a.hydrated.due("channel:"+channelID, time.Now()) {
			if channel, err := a.session.Channel(channelID); err == nil {
				_ = state.ChannelAdd(channel)
			}
		}
	}
}

// retryGate admits one attempt per key per discordHydrateRetry, so a channel
// REST cannot read costs one call per interval rather than one per message.
type retryGate struct {
	mu   sync.Mutex
	last map[string]time.Time
}

func (g *retryGate) due(key string, now time.Time) bool {
	g.mu.Lock()
	defer g.mu.Unlock()
	if g.last == nil {
		g.last = map[string]time.Time{}
	}
	if tried, seen := g.last[key]; seen && now.Sub(tried) < discordHydrateRetry {
		return false
	}
	g.last[key] = now
	return true
}
