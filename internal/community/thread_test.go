package community

import (
	"strings"
	"testing"
	"time"

	"github.com/bwmarrin/discordgo"
)

// A long answer gets somewhere of its own, and a thread that cannot be made
// must never cost a member their reply. See sirens-echo#239.

func TestAThreadNameComesFromTheMemberNotFromUs(t *testing.T) {
	t.Parallel()
	message := &discordgo.Message{Content: "what is the Eco server status right now"}
	if got := threadNameFor(message); got != "what is the Eco server status right now" {
		t.Errorf("thread name = %q", got)
	}
}

// Markup and mentions are dropped rather than summarised, because summarising
// would be authoring a name for a member.
func TestAThreadNameDropsMarkupAndNeverEmpties(t *testing.T) {
	t.Parallel()
	got := threadNameFor(&discordgo.Message{Content: "**hey** <@123>, status?"})
	for _, banned := range []string{"*", "<", "@", "?"} {
		if strings.Contains(got, banned) {
			t.Errorf("%q survived into the thread name %q", banned, got)
		}
	}
	if strings.Contains(got, "  ") {
		t.Errorf("dropped markup left a double space: %q", got)
	}
	// All punctuation still has to produce a name, since Discord refuses a
	// thread without one.
	if name := threadNameFor(&discordgo.Message{Content: "!!! ???"}); name != threadNameFallback {
		t.Errorf("an unusable message produced %q, not the fallback", name)
	}
	if threadNameFor(nil) != threadNameFallback {
		t.Error("a nil message produced no fallback name")
	}
}

func TestAThreadNameStaysInsideDiscordsCap(t *testing.T) {
	t.Parallel()
	long := strings.Repeat("status ", 60)
	if got := threadNameFor(&discordgo.Message{Content: long}); len([]rune(got)) > threadNameRunes {
		t.Errorf("thread name is %d runes, over Discord's cap of %d", len([]rune(got)), threadNameRunes)
	}
}

// The safety property. Every failure path returns "no thread" rather than an
// error, so the caller replies in the channel exactly as it does today.
func TestAThreadThatCannotBeMadeCostsNothing(t *testing.T) {
	t.Parallel()
	if _, threaded := threadForReply(nil, &discordgo.Message{ID: "1"}); threaded {
		t.Error("a nil session reported a thread")
	}
	session := statefulSession(t)
	if _, threaded := threadForReply(session, nil); threaded {
		t.Error("a nil message reported a thread")
	}
	// A turn already in a thread does not nest. Resolved from cached state, so
	// this makes no API call and the unreachable session is never dialled.
	thread := &discordgo.Channel{
		ID:       "1390000000000000009",
		ParentID: "1390000000000000003",
		Type:     discordgo.ChannelTypeGuildPublicThread,
		GuildID:  "1390000000000000002",
	}
	if err := session.State.ChannelAdd(thread); err != nil {
		t.Fatalf("seed channel state: %v", err)
	}
	inThread := &discordgo.Message{ID: "1401", ChannelID: thread.ID, Content: "again"}
	if _, threaded := threadForReply(session, inThread); threaded {
		t.Error("a turn inside a thread started a nested one")
	}
}

// A turn that never posted a progress line has nothing in the channel pointing
// at a thread, so it does not get one however long it took.
func TestATurnWithNoProgressLineGetsNoThread(t *testing.T) {
	t.Parallel()
	var quiet *turnProgress
	if quiet.longEnough() {
		t.Error("a nil progress reported a long turn")
	}
	if turnLongReply(t.Context()) {
		t.Error("a context with no progress reported a long turn")
	}
}

// The window is Kai's: the wait plus two beats, which the progress line's own
// grid already measures. See sirens-echo#354.
func TestTheThreadWindowIsTheOneThatWasAskedFor(t *testing.T) {
	t.Parallel()
	if turnLongReplyAfter != 15*time.Second {
		t.Errorf("the thread window is %s, want the 3 + 6 + 6 that was asked for", turnLongReplyAfter)
	}
	if turnLongReplyAfter <= turnProgressAfter {
		t.Error("a turn could cross the thread window before it posts a progress line")
	}
}
