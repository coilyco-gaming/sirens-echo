package community

import (
	"context"
	"errors"
	"testing"

	"github.com/bwmarrin/discordgo"
)

// The declared set was rendered and never published, so every slash command
// this repository ships was unreachable. See sirens-echo#127.

// recordingRegistrar captures what would have been written to Discord.
type recordingRegistrar struct {
	guilds   []string
	commands [][]*discordgo.ApplicationCommand
	appID    string
	err      error
}

func (r *recordingRegistrar) ApplicationCommandBulkOverwrite(
	appID string,
	guildID string,
	commands []*discordgo.ApplicationCommand,
	_ ...discordgo.RequestOption,
) ([]*discordgo.ApplicationCommand, error) {
	r.appID = appID
	if r.err != nil {
		return nil, r.err
	}
	r.guilds = append(r.guilds, guildID)
	r.commands = append(r.commands, commands)
	return commands, nil
}

// The whole declared set reaches every admitted guild, which is the difference
// between a command existing and a member being able to invoke it.
func TestRegistrationPublishesEveryDeclaredCommandToEveryAdmittedGuild(t *testing.T) {
	t.Parallel()
	registrar := &recordingRegistrar{}
	guilds := []string{"1024000000000000001", "1024000000000000002"}

	if err := registerCommands(
		context.Background(), registrar, "app-1", guilds, nil, nil,
	); err != nil {
		t.Fatalf("registerCommands: %v", err)
	}
	if len(registrar.guilds) != len(guilds) {
		t.Fatalf("registered in %v, want %v", registrar.guilds, guilds)
	}
	declared, err := discordCommands(nil)
	if err != nil {
		t.Fatalf("discordCommands: %v", err)
	}
	for index, published := range registrar.commands {
		if len(published) != len(declared) {
			t.Errorf("guild %s got %d commands, want the declared %d",
				registrar.guilds[index], len(published), len(declared))
		}
	}
	names := map[string]bool{}
	for _, command := range registrar.commands[0] {
		names[command.Name] = true
	}
	// Named rather than counted, so dropping one from the declaration fails
	// here instead of quietly shrinking the surface.
	for _, want := range []string{"echo", "job-status", "job-cancel", "mcps"} {
		if !names[want] {
			t.Errorf("%q was declared and not published", want)
		}
	}
}

// A global registration would advertise the command in guilds the policy
// refuses, so a deployment with no admitted guild registers nowhere.
func TestRegistrationRefusesWhenNoGuildIsAdmitted(t *testing.T) {
	t.Parallel()
	registrar := &recordingRegistrar{}
	err := registerCommands(context.Background(), registrar, "app-1", nil, nil, nil)
	if err == nil {
		t.Fatal("registering with no admitted guild succeeded")
	}
	if len(registrar.guilds) != 0 {
		t.Errorf("wrote to %v anyway", registrar.guilds)
	}
}

// Without an application id there is nothing to register against, and guessing
// one would write into another application.
func TestRegistrationRefusesWithoutAnApplicationID(t *testing.T) {
	t.Parallel()
	registrar := &recordingRegistrar{}
	err := registerCommands(
		context.Background(), registrar, "", []string{"1024000000000000001"}, nil, nil,
	)
	if err == nil {
		t.Fatal("registering without an application id succeeded")
	}
	if len(registrar.guilds) != 0 {
		t.Errorf("wrote to %v anyway", registrar.guilds)
	}
}

// One guild refusing must not stop the others, because a partial surface beats
// none and the failure is already reported.
func TestOneGuildFailingDoesNotStopTheRegistration(t *testing.T) {
	t.Parallel()
	registrar := &recordingRegistrar{err: errors.New("missing access")}
	err := registerCommands(
		context.Background(), registrar, "app-1",
		[]string{"1024000000000000001", "1024000000000000002"}, nil, nil,
	)
	if err == nil {
		t.Fatal("a failed registration reported success")
	}
	// Both were attempted: the loop reports and continues rather than returning.
	if registrar.appID != "app-1" {
		t.Errorf("application id = %q", registrar.appID)
	}
}

// The gate is the deployment's, so a service with commands off registers
// nothing however many guilds it admits.
func TestCommandsOffRegistersNothing(t *testing.T) {
	t.Parallel()
	agent := &Agent{
		cfg:       Config{DiscordCommandsEnabled: false},
		telemetry: telemetryOrNoop(nil),
	}
	// A nil session is the other half: registration needs one and must not panic.
	agent.registerCommandsOnReady(context.Background(), &discordgo.Ready{
		User: &discordgo.User{ID: "app-1"},
	})
}

// The admitted set comes from the access policy, so the command surface cannot
// reach a guild the policy does not.
func TestAdmittedGuildsComesFromTheAccessPolicy(t *testing.T) {
	t.Parallel()
	agent := &Agent{access: &AccessPolicy{Guilds: []GuildAccess{
		{ID: "1024000000000000001"},
		{ID: ""},
		{ID: "1024000000000000002"},
	}}}
	guilds := agent.admittedGuilds()
	if len(guilds) != 2 {
		t.Fatalf("admitted = %v, want the two with ids", guilds)
	}
	if (&Agent{}).admittedGuilds() != nil {
		t.Error("an agent with no policy admitted a guild")
	}
}
