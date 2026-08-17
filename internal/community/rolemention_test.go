package community

import (
	"testing"

	"github.com/bwmarrin/discordgo"
)

// roleSession is a guild where this account holds one role, which is what a
// member @s when they address the service by role. See sirens-echo#866.
func roleSession(t *testing.T, botID, guildID string, roles []string) *discordgo.Session {
	t.Helper()
	state := discordgo.NewState()
	state.User = &discordgo.User{ID: botID}
	if err := state.GuildAdd(&discordgo.Guild{ID: guildID}); err != nil {
		t.Fatalf("GuildAdd: %v", err)
	}
	if err := state.MemberAdd(&discordgo.Member{
		GuildID: guildID,
		User:    &discordgo.User{ID: botID},
		Roles:   roles,
	}); err != nil {
		t.Fatalf("MemberAdd: %v", err)
	}
	return &discordgo.Session{State: state}
}

// The ask: a mention of the role summons, not only a mention of the account.
func TestARoleMentionSummons(t *testing.T) {
	t.Parallel()
	session := roleSession(t, "bot-1", "guild-1", []string{"role-agents"})

	summoned, lookup := summonedLocally(session, &discordgo.Message{
		GuildID:      "guild-1",
		ChannelID:    "channel-1",
		Author:       &discordgo.User{ID: "member-1"},
		MentionRoles: []string{"role-agents"},
	})
	if !summoned || lookup {
		t.Fatalf("a mention of the held role did not summon: summoned=%v lookup=%v",
			summoned, lookup)
	}
}

// A role this account does not hold is somebody else being addressed.
func TestAnotherRolesMentionDoesNotSummon(t *testing.T) {
	t.Parallel()
	session := roleSession(t, "bot-1", "guild-1", []string{"role-agents"})

	summoned, lookup := summonedLocally(session, &discordgo.Message{
		GuildID:      "guild-1",
		ChannelID:    "channel-1",
		Author:       &discordgo.User{ID: "member-1"},
		MentionRoles: []string{"role-moderators"},
	})
	if summoned || lookup {
		t.Fatalf("a role this account does not hold summoned: summoned=%v", summoned)
	}
}

// The everyone role's id is the guild's, and every member holds it. An
// announcement must not summon every agent in the channel.
func TestAnEveryoneMentionDoesNotSummon(t *testing.T) {
	t.Parallel()
	session := roleSession(t, "bot-1", "guild-1", []string{"guild-1", "role-agents"})

	summoned, _ := summonedLocally(session, &discordgo.Message{
		GuildID:         "guild-1",
		ChannelID:       "channel-1",
		Author:          &discordgo.User{ID: "member-1"},
		MentionRoles:    []string{"guild-1"},
		MentionEveryone: true,
	})
	if summoned {
		t.Error("an @everyone announcement summoned the service")
	}
}

// An ordinary message must make no lookup at all, so the quiet path stays quiet.
func TestAMessageWithNoRoleMentionReadsNoMember(t *testing.T) {
	t.Parallel()
	// No member in state, so any lookup would fall through to a REST call and
	// a nil session would panic rather than passing quietly.
	state := discordgo.NewState()
	state.User = &discordgo.User{ID: "bot-1"}
	session := &discordgo.Session{State: state}

	summoned, lookup := summonedLocally(session, &discordgo.Message{
		GuildID:   "guild-1",
		ChannelID: "channel-1",
		Author:    &discordgo.User{ID: "member-1"},
	})
	if summoned || lookup {
		t.Fatalf("an unmentioned message summoned: summoned=%v lookup=%v", summoned, lookup)
	}
}
