package community

import (
	"testing"

	"github.com/bwmarrin/discordgo"
	"go.opentelemetry.io/otel/attribute"
)

// A trace id a member was handed is only worth opening if the turn it names
// can be placed. See sirens-echo#337.

func turnAttributes(t *testing.T, turn *discordMessageTurn) map[string]string {
	t.Helper()
	rendered := map[string]string{}
	for _, pair := range turn.SpanAttributes() {
		rendered[string(pair.Key)] = pair.Value.AsString()
	}
	return rendered
}

// statefulSession is a session that never dials: only its channel state is
// read, and the point of the code under test is that it makes no API call.
func statefulSession(t *testing.T) *discordgo.Session {
	t.Helper()
	session, err := discordgo.New("")
	if err != nil {
		t.Fatalf("build session: %v", err)
	}
	session.State = discordgo.NewState()
	// A channel only enters the cache under a guild it belongs to, which is the
	// same shape the gateway delivers it in.
	if err := session.State.GuildAdd(&discordgo.Guild{ID: "1390000000000000002"}); err != nil {
		t.Fatalf("seed guild state: %v", err)
	}
	return session
}

func guildTurn(session *discordgo.Session, channelID string) *discordMessageTurn {
	return &discordMessageTurn{
		session: session,
		message: &discordgo.Message{
			ID:        "1401110000000000001",
			GuildID:   "1390000000000000002",
			ChannelID: channelID,
			Author:    &discordgo.User{ID: "318190481467244544"},
		},
	}
}

func TestTheTurnSpanCarriesTheDiscordIdentifiers(t *testing.T) {
	t.Parallel()
	rendered := turnAttributes(t, guildTurn(nil, "1390000000000000003"))
	for key, want := range map[string]string{
		"discord.user.id":          "318190481467244544",
		"discord.guild.id":         "1390000000000000002",
		"discord.channel.id":       "1390000000000000003",
		"messaging.message.id":     "1401110000000000001",
		"messaging.system":         "discord",
		"messaging.operation.name": "receive",
	} {
		if rendered[key] != want {
			t.Errorf("%s = %q, want %q", key, rendered[key], want)
		}
	}
	// Absent rather than empty. A blank thread id reads as a thread nobody can
	// find, which is worse than no answer.
	if _, present := rendered["discord.thread.id"]; present {
		t.Error("a turn outside a thread reported a thread id")
	}
}

// Discord models a thread as a channel. Reporting the thread as the channel
// would hide every thread turn from a query for the channel it hangs under.
func TestAThreadTurnReportsBothTheThreadAndItsParent(t *testing.T) {
	t.Parallel()
	session := statefulSession(t)
	thread := &discordgo.Channel{
		ID:       "1390000000000000009",
		ParentID: "1390000000000000003",
		Type:     discordgo.ChannelTypeGuildPublicThread,
		GuildID:  "1390000000000000002",
	}
	if err := session.State.ChannelAdd(thread); err != nil {
		t.Fatalf("seed channel state: %v", err)
	}
	rendered := turnAttributes(t, guildTurn(session, thread.ID))
	if rendered["discord.thread.id"] != thread.ID {
		t.Errorf("thread id = %q, want %q", rendered["discord.thread.id"], thread.ID)
	}
	if rendered["discord.channel.id"] != thread.ParentID {
		t.Errorf("channel id = %q, want the parent %q", rendered["discord.channel.id"], thread.ParentID)
	}
}

// The boundary spans sit in the same trace as the turn span, so a channel that
// means the parent on one and the thread on another makes both queries partial.
func TestEveryBoundarySpanReportsTheParentForAThreadTurn(t *testing.T) {
	t.Parallel()
	session := statefulSession(t)
	thread := &discordgo.Channel{
		ID:       "1390000000000000009",
		ParentID: "1390000000000000003",
		Type:     discordgo.ChannelTypeGuildPublicThread,
		GuildID:  "1390000000000000002",
	}
	if err := session.State.ChannelAdd(thread); err != nil {
		t.Fatalf("seed channel state: %v", err)
	}
	message := guildTurn(session, thread.ID).message
	for _, operation := range []string{"process", "send"} {
		rendered := map[string]string{}
		for _, pair := range discordMessageSpanAttributes(
			operation,
			discordLocationFor(session, message),
			"",
		) {
			rendered[string(pair.Key)] = pair.Value.AsString()
		}
		if rendered["discord.channel.id"] != thread.ParentID {
			t.Errorf(
				"%s channel id = %q, want the parent %q",
				operation, rendered["discord.channel.id"], thread.ParentID,
			)
		}
		if rendered["discord.thread.id"] != thread.ID {
			t.Errorf("%s thread id = %q, want %q", operation, rendered["discord.thread.id"], thread.ID)
		}
	}
}

// An unresolved channel reports itself as a channel, which is what it is. The
// alternative is a Discord API call per turn to learn something optional.
func TestAnUnresolvedChannelCostsNoAPICall(t *testing.T) {
	t.Parallel()
	session := statefulSession(t)
	rendered := turnAttributes(t, guildTurn(session, "1390000000000000044"))
	if rendered["discord.channel.id"] != "1390000000000000044" {
		t.Errorf("channel id = %q", rendered["discord.channel.id"])
	}
	if _, present := rendered["discord.thread.id"]; present {
		t.Error("an unresolved channel invented a thread id")
	}
}

// A DM never enters the turn logger, and this change is not the first
// exception to that. See AGENTS.md.
func TestADirectMessageContributesNoIdentifiers(t *testing.T) {
	t.Parallel()
	turn := guildTurn(nil, "1390000000000000003")
	turn.message.GuildID = ""
	if got := turn.SpanAttributes(); len(got) != 0 {
		t.Errorf("a direct message contributed %d attributes: %v", len(got), got)
	}
}

// The HTTP profile has no Discord identity, and asserting that keeps the
// optional interface optional rather than something every transport grows.
func TestTheHTTPTurnContributesNothing(t *testing.T) {
	t.Parallel()
	var turn turnIO = &httpTurn{requestID: "req-1", requester: "operator"}
	if _, tags := turn.(spanTagger); tags {
		t.Error("the HTTP turn implements spanTagger, so it now claims identifiers it has none of")
	}
}

// The account id is here by an explicit reversal. Removing it means deleting
// this assertion in the same commit, which is the point.
func TestTheAccountIDIsPresentByDecisionAndNotByAccident(t *testing.T) {
	t.Parallel()
	rendered := turnAttributes(t, guildTurn(nil, "1390000000000000003"))
	if rendered["discord.user.id"] == "" {
		t.Fatal("the account id left the turn span with no accompanying doc change")
	}
	var keys []attribute.Key
	for _, pair := range guildTurn(nil, "1390000000000000003").SpanAttributes() {
		keys = append(keys, pair.Key)
	}
	// The reply body is a separate boundary and stays closed: identifiers go to
	// telemetry, never into what a member reads. See sirens-echo#310.
	for _, key := range keys {
		if key == "discord.user.name" || key == "discord.user.handle" {
			t.Errorf("a member-visible name reached the span as %s", key)
		}
	}
}
