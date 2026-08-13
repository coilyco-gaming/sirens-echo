package community

import (
	"testing"

	"github.com/bwmarrin/discordgo"
)

// The progress line was a bare channel post while the reply that replaces it
// was a reply. See sirens-echo#376.

func TestTheProgressLineReferencesTheMemberMessage(t *testing.T) {
	t.Parallel()
	message := &discordgo.Message{
		ID:        "1401110000000000001",
		ChannelID: "1390000000000000003",
		GuildID:   "1390000000000000002",
	}
	sink := discordTurnProgress{channel: message.ChannelID, message: message}
	reference := sink.reference()
	if reference == nil {
		t.Fatal("the progress line carries no reference, so it posts bare")
	}
	if reference.MessageID != message.ID {
		t.Errorf("reference names %q, want the member's message %q", reference.MessageID, message.ID)
	}
	if reference.ChannelID != message.ChannelID {
		t.Errorf("reference channel = %q", reference.ChannelID)
	}
}

// The turn's own reply already referenced the message. Both halves of one
// exchange should point at the same thing.
func TestTheLineAndTheReplyReferenceTheSameMessage(t *testing.T) {
	t.Parallel()
	message := &discordgo.Message{
		ID:        "1401110000000000001",
		ChannelID: "1390000000000000003",
		GuildID:   "1390000000000000002",
	}
	line := discordTurnProgress{channel: message.ChannelID, message: message}.reference()
	reply := message.SoftReference()
	if line.MessageID != reply.MessageID || line.ChannelID != reply.ChannelID {
		t.Errorf("line references %+v, reply references %+v", line, reply)
	}
}

// A sink with no message still posts, rather than failing to narrate at all.
// Losing the progress line would be a worse outcome than losing the reference.
func TestASinkWithNoMessageStillPosts(t *testing.T) {
	t.Parallel()
	if reference := (discordTurnProgress{channel: "1390000000000000003"}).reference(); reference != nil {
		t.Errorf("a sink with no message invented a reference: %+v", reference)
	}
}
