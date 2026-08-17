package community

import (
	"context"
	"fmt"
	"log/slog"

	"github.com/bwmarrin/discordgo"
)

// Publishing the declared set to Discord, which is what makes a slash command
// reachable at all. See docs/sirens-echo-commands.md.

// commandRegistrar is the write this needs, so a test can observe the
// registration without a Discord session.
type commandRegistrar interface {
	ApplicationCommandBulkOverwrite(
		appID string,
		guildID string,
		commands []*discordgo.ApplicationCommand,
		options ...discordgo.RequestOption,
	) ([]*discordgo.ApplicationCommand, error)
}

// registerCommands publishes the declared set to every admitted guild. Per
// guild, not globally. See docs/sirens-echo-commands.md.
func registerCommands(
	ctx context.Context,
	registrar commandRegistrar,
	appID string,
	guilds []string,
	telemetry *Telemetry,
) error {
	telemetry = telemetryOrNoop(telemetry)
	commands, err := discordCommands()
	if err != nil {
		return fmt.Errorf("render commands: %w", err)
	}
	if appID == "" {
		return fmt.Errorf("no application id, so commands cannot be registered")
	}
	if len(guilds) == 0 {
		// Refused rather than falling back to a global registration, which would
		// publish into guilds this deployment does not admit.
		return fmt.Errorf("no admitted guild, so there is nowhere to register")
	}
	var failed error
	for _, guild := range guilds {
		// A bulk overwrite, so a command removed from the declaration disappears
		// from Discord rather than lingering as an invocable ghost.
		created, err := registrar.ApplicationCommandBulkOverwrite(appID, guild, commands)
		if err != nil {
			telemetry.Error(ctx, "discord.commands.failed",
				slog.String("guild", guild), slog.Int("commands", len(commands)))
			if failed == nil {
				failed = fmt.Errorf("register commands in guild %s: %w", guild, err)
			}
			continue
		}
		telemetry.Info(ctx, "discord.commands.registered",
			slog.String("guild", guild), slog.Int("commands", len(created)))
	}
	return failed
}

// admittedGuilds lists the guilds this deployment answers in, which is the set
// a command may be published to.
func (a *Agent) admittedGuilds() []string {
	if a.access == nil {
		return nil
	}
	guilds := make([]string, 0, len(a.access.Guilds))
	for _, guild := range a.access.Guilds {
		if guild.ID != "" {
			guilds = append(guilds, guild.ID)
		}
	}
	return guilds
}

// registerCommandsOnReady publishes the set once the gateway has told us who we
// are. A failure is logged and never fatal: the rest of the service works.
func (a *Agent) registerCommandsOnReady(ctx context.Context, ready *discordgo.Ready) {
	if !a.cfg.DiscordCommandsEnabled || a.session == nil || ready == nil ||
		ready.User == nil {
		return
	}
	if err := registerCommands(
		ctx, a.session, ready.User.ID, a.admittedGuilds(), a.telemetry,
	); err != nil {
		a.telemetry.Error(ctx, "discord.commands.registration_failed",
			slog.String("error", err.Error()))
	}
}
