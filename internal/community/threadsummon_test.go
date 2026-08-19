package community

import (
	"testing"

	"github.com/bwmarrin/discordgo"
)

// threadSummonSession is a session whose state holds one guild, one ordinary
// channel, and one thread owned by the named account.
func threadSummonSession(t *testing.T, botID, threadOwner string) *discordgo.Session {
	t.Helper()
	session := editSession(botID)
	if err := session.State.GuildAdd(&discordgo.Guild{ID: "guild-1"}); err != nil {
		t.Fatalf("guild add: %v", err)
	}
	channels := []*discordgo.Channel{
		{ID: "channel-1", GuildID: "guild-1", Type: discordgo.ChannelTypeGuildText},
		{
			ID:      "thread-1",
			GuildID: "guild-1",
			Type:    discordgo.ChannelTypeGuildPublicThread,
			OwnerID: threadOwner,
		},
	}
	for _, channel := range channels {
		if err := session.State.ChannelAdd(channel); err != nil {
			t.Fatalf("channel add %s: %v", channel.ID, err)
		}
	}
	return session
}

// A thread this service opened is its own conversation, so an ordinary message
// in it summons. See sirens-echo#750.
func TestMessageInServiceOwnedThreadSummons(t *testing.T) {
	t.Parallel()
	const botID = "bot-1"
	session := threadSummonSession(t, botID, botID)

	summoned, lookup := summonedLocally(session, &discordgo.Message{
		GuildID:   "guild-1",
		ChannelID: "thread-1",
		Author:    &discordgo.User{ID: "member-1"},
	})
	if !summoned || lookup {
		t.Fatalf(
			"a message in a service-owned thread must summon, got summoned=%v lookup=%v",
			summoned, lookup,
		)
	}

	// Any member's message, not only the one the thread was opened for. This is
	// the accepted consequence rather than an oversight.
	summoned, lookup = summonedLocally(session, &discordgo.Message{
		GuildID:           "guild-1",
		ChannelID:         "thread-1",
		Author:            &discordgo.User{ID: "member-2"},
		ReferencedMessage: &discordgo.Message{Author: &discordgo.User{ID: "member-1"}},
	})
	if !summoned || lookup {
		t.Fatalf(
			"one member answering another in a service thread must summon, got summoned=%v lookup=%v",
			summoned, lookup,
		)
	}
}

// Ownership is the whole signal, so a thread a member opened keeps the mention
// gate that keeps a busy channel quiet.
func TestMessageInMemberOwnedThreadDoesNotSummon(t *testing.T) {
	t.Parallel()
	const botID = "bot-1"
	session := threadSummonSession(t, botID, "member-1")

	summoned, lookup := summonedLocally(session, &discordgo.Message{
		GuildID:   "guild-1",
		ChannelID: "thread-1",
		Author:    &discordgo.User{ID: "member-1"},
	})
	if summoned || lookup {
		t.Fatalf(
			"a member-created thread must not summon on its own, got summoned=%v lookup=%v",
			summoned, lookup,
		)
	}

	// A mention still works inside it, unchanged.
	summoned, lookup = summonedLocally(session, &discordgo.Message{
		GuildID:   "guild-1",
		ChannelID: "thread-1",
		Author:    &discordgo.User{ID: "member-1"},
		Mentions:  []*discordgo.User{{ID: botID}},
	})
	if !summoned || lookup {
		t.Fatalf("a mention must still summon in a member thread, got summoned=%v lookup=%v",
			summoned, lookup)
	}
}

// An ordinary channel is never a thread, whoever owns it, so the parent channel
// of a service thread stays gated.
func TestOrdinaryChannelIsNotSummonedByOwnership(t *testing.T) {
	t.Parallel()
	const botID = "bot-1"
	session := threadSummonSession(t, botID, botID)

	summoned, lookup := summonedLocally(session, &discordgo.Message{
		GuildID:   "guild-1",
		ChannelID: "channel-1",
		Author:    &discordgo.User{ID: "member-1"},
	})
	if summoned || lookup {
		t.Fatalf("an unmentioned channel message must not summon, got summoned=%v lookup=%v",
			summoned, lookup)
	}
}

// A thread the gateway never delivered is unknown rather than owned, and the
// decision stays free of a Discord call.
func TestUnknownThreadFallsThroughToTheOtherSignals(t *testing.T) {
	t.Parallel()
	const botID = "bot-1"
	session := threadSummonSession(t, botID, botID)

	summoned, lookup := summonedLocally(session, &discordgo.Message{
		GuildID:   "guild-1",
		ChannelID: "thread-unseen",
		Author:    &discordgo.User{ID: "member-1"},
	})
	if summoned || lookup {
		t.Fatalf("an unknown channel must not summon by ownership, got summoned=%v lookup=%v",
			summoned, lookup)
	}

	// The reply signal still decides for it, so an unseen thread is not deaf.
	summoned, lookup = summonedLocally(session, &discordgo.Message{
		GuildID:           "guild-1",
		ChannelID:         "thread-unseen",
		Author:            &discordgo.User{ID: "member-1"},
		ReferencedMessage: &discordgo.Message{Author: &discordgo.User{ID: botID}},
	})
	if !summoned || lookup {
		t.Fatalf("a reply in an unseen thread must still summon, got summoned=%v lookup=%v",
			summoned, lookup)
	}
}

// The measurement sirens-echo#750 asked for, held as a test: the gateway state
// carries a thread channel and its creator, so ownership costs no REST call.
func TestThreadOwnershipReadsCachedGatewayState(t *testing.T) {
	t.Parallel()
	const botID = "bot-1"
	// A session with no HTTP client at all, so any REST fallback would panic
	// or fail rather than quietly costing a call per message.
	session := threadSummonSession(t, botID, botID)

	if !threadOwnedBy(session, "thread-1", botID) {
		t.Fatal("a cached thread must report its owner without a Discord call")
	}
	if threadOwnedBy(session, "channel-1", botID) {
		t.Fatal("an ordinary channel is not a thread")
	}
	if threadOwnedBy(session, "thread-unseen", botID) {
		t.Fatal("an uncached channel must report unowned rather than looking it up")
	}
	if threadOwnedBy(&discordgo.Session{}, "thread-1", botID) {
		t.Fatal("a session with no state must report unowned")
	}
}

// The resolver's cache, state read, and negative answer. The REST fallback
// needs an HTTP fixture and is exercised live instead.
func testAgentForOwnership(t *testing.T) *Agent {
	t.Helper()
	agent := &Agent{}
	agent.ensureRuntimeDefaults()
	return agent
}

func TestAResolvedThreadSummonsOnALaterMessage(t *testing.T) {
	botID := "bot-1"
	session := threadSummonSession(t, botID, botID)
	agent := testAgentForOwnership(t)

	// State alone cannot answer for a thread it never saw.
	if threadOwnedBy(session, "thread-unseen", botID) {
		t.Fatal("cached state should not claim an unseen thread")
	}

	agent.threads.Set("thread-unseen", true)
	if !agent.resolveThreadOwnership(session, summonContext{ChannelID: "thread-unseen"}) {
		t.Fatal("a resolved thread should summon on a later message")
	}
}

func TestResolvedOwnershipIsCachedSoALookupIsPerChannel(t *testing.T) {
	botID := "bot-1"
	session := threadSummonSession(t, botID, botID)
	agent := testAgentForOwnership(t)

	origin := summonContext{ChannelID: "thread-1", GuildID: "guild-1"}
	if !agent.resolveThreadOwnership(session, origin) {
		t.Fatal("a thread in state and owned by this account should summon")
	}
	owned, known := agent.threads.Get("thread-1")
	if !known || !owned {
		t.Fatalf("ownership should be cached: known=%v owned=%v", known, owned)
	}
}

func TestOrdinaryChannelCachesFalseRatherThanLookingUpEachMessage(t *testing.T) {
	botID := "bot-1"
	session := threadSummonSession(t, botID, botID)
	agent := testAgentForOwnership(t)

	origin := summonContext{ChannelID: "channel-1", GuildID: "guild-1"}
	if agent.resolveThreadOwnership(session, origin) {
		t.Fatal("an ordinary channel is not a thread this service opened")
	}
	owned, known := agent.threads.Get("channel-1")
	if !known || owned {
		t.Fatalf("the negative answer should cache: known=%v owned=%v", known, owned)
	}
}

func TestAMemberOwnedThreadStaysUnowned(t *testing.T) {
	botID := "bot-1"
	session := threadSummonSession(t, botID, "member-9")
	agent := testAgentForOwnership(t)

	origin := summonContext{ChannelID: "thread-1", GuildID: "guild-1"}
	if agent.resolveThreadOwnership(session, origin) {
		t.Fatal("a thread a member opened must not summon")
	}
}
