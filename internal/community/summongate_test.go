package community

import (
	"strings"
	"testing"

	"github.com/bwmarrin/discordgo"
)

// summonedBool reads the gate as the yes or no the older assertions were
// written against, the gate itself now reporting why. See sirens-echo#992.
func summonedBool(reason summonReason, lookup bool) (bool, bool) {
	return reason.summoned(), lookup
}

// The complaint. A message that never summoned produced no reply, correctly,
// and also no record, so it was indistinguishable from a turn that died.
func TestTheSummonGateNamesWhyItRefused(t *testing.T) {
	t.Parallel()
	const botID = "bot-1"
	// A thread owned by a member rather than by this service, which is the
	// shape members report as the service ignoring them.
	session := threadSummonSession(t, botID, "member-1")
	author := &discordgo.User{ID: "member-1"}

	cases := []struct {
		name    string
		message *discordgo.Message
		want    summonReason
	}{
		{
			"an ordinary channel message nobody addressed",
			&discordgo.Message{GuildID: "guild-1", ChannelID: "channel-1", Author: author},
			summonNotAddressed,
		},
		{
			"a message inside a thread, which is the one people report",
			&discordgo.Message{GuildID: "guild-1", ChannelID: "thread-1", Author: author},
			summonNotAddressedThread,
		},
		{
			"a reply to somebody else",
			&discordgo.Message{
				GuildID: "guild-1", ChannelID: "channel-1", Author: author,
				ReferencedMessage: &discordgo.Message{Author: &discordgo.User{ID: "member-2"}},
			},
			summonReplyToAnother,
		},
	}
	for _, test := range cases {
		t.Run(test.name, func(t *testing.T) {
			reason, _ := summonedLocally(session, test.message)
			if reason != test.want {
				t.Errorf("reason = %q, want %q", reason, test.want)
			}
			if reason.summoned() {
				t.Errorf("reason %q admitted a message nobody addressed", reason)
			}
		})
	}
}

// The refusal a member reports and the refusal a thread produces must not
// collapse, because they have opposite fixes: #750 against #939.
func TestAThreadRefusalIsNotAnOrdinaryChannelRefusal(t *testing.T) {
	t.Parallel()
	if summonNotAddressedThread == summonNotAddressed {
		t.Fatal("the thread refusal and the channel refusal are one label")
	}
}

// Every admitting reason has to answer yes and every refusing one no, or the
// gate admits on a label rather than on a decision.
func TestTheSummonReasonSetSplitsCleanly(t *testing.T) {
	t.Parallel()
	for _, reason := range []summonReason{
		summonDirect, summonMentioned, summonOwnedThread, summonRepliedTo,
	} {
		if !reason.summoned() {
			t.Errorf("%q does not admit, so an addressed message is dropped", reason)
		}
	}
	for _, reason := range []summonReason{
		summonNotAddressed, summonNotAddressedThread,
		summonReplyToAnother, summonReferenceUnknown,
	} {
		if reason.summoned() {
			t.Errorf("%q admits, so an unaddressed message becomes a turn", reason)
		}
	}
}

// A reason carrying a channel, guild, author, or message id would let a
// flooder grow the metric by opening channels.
func TestNoSummonReasonCarriesAnIdentifier(t *testing.T) {
	t.Parallel()
	const botID = "bot-1"
	session := threadSummonSession(t, botID, "member-1")
	message := &discordgo.Message{
		ID: "message-1", GuildID: "guild-1", ChannelID: "thread-1",
		Author: &discordgo.User{ID: "member-1"},
	}
	reason, _ := summonedLocally(session, message)
	for _, identifier := range []string{
		message.ID, message.GuildID, message.ChannelID, message.Author.ID,
	} {
		if identifier != "" && strings.Contains(string(reason), identifier) {
			t.Errorf("reason %q carries %q, which is unbounded", reason, identifier)
		}
	}
}
