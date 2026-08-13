package community

import (
	"context"
	"errors"
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
	if _, threaded := threadForReply(nil, &discordgo.Message{ID: "1"}, ""); threaded {
		t.Error("a nil session reported a thread")
	}
	session := statefulSession(t)
	if _, threaded := threadForReply(session, nil, ""); threaded {
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
	if _, threaded := threadForReply(session, inThread, ""); threaded {
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

// The window is the wait plus two beats, and it moved with the wait on
// sirens-echo#375. See docs/sirens-echo-progress-cadence.md.
func TestTheThreadWindowIsTheOneThatWasAskedFor(t *testing.T) {
	t.Parallel()
	if turnLongReplyAfter != 25*time.Second {
		t.Errorf("the thread window is %s, want the wait plus two beats", turnLongReplyAfter)
	}
	if turnLongReplyAfter <= turnProgressAfter {
		t.Error("a turn could cross the thread window before it posts a progress line")
	}
}

// titlingClient answers with a fixed summary, or fails, so the fallback is
// testable without a model. See sirens-echo#461.
type titlingClient struct {
	reply string
	fail  bool
}

func (c titlingClient) Complete(
	context.Context, TurnPrompt, string,
) (CompletionResult, error) {
	if c.fail {
		return CompletionResult{}, errTitleUnavailable
	}
	return CompletionResult{Content: c.reply}, nil
}

var errTitleUnavailable = errors.New("no title")

func TestAThreadTitleSummarisesTheIntent(t *testing.T) {
	t.Parallel()
	message := &discordgo.Message{Content: "how much does it cost to build a log house"}
	got := threadTitle(t.Context(), titlingClient{reply: "log house pricing"}, message, "req-1")
	if got != "log house pricing" {
		t.Errorf("title = %q, want the summary", got)
	}
}

// A title that fails must not cost a member their thread.
func TestAFailedTitleKeepsTheDerivedName(t *testing.T) {
	t.Parallel()
	message := &discordgo.Message{Content: "how much does it cost to build a log house"}
	if got := threadTitle(t.Context(), titlingClient{fail: true}, message, "req-1"); got != "" {
		t.Errorf("a failed title returned %q, want empty so the caller falls back", got)
	}
	if got := threadTitle(t.Context(), nil, message, "req-1"); got != "" {
		t.Errorf("a nil client returned %q", got)
	}
	// The fallback is still the derived name, unchanged.
	if got := threadNameFor(message); got != "how much does it cost to build a log house" {
		t.Errorf("the derived name changed: %q", got)
	}
}

// A summary takes the same cleaning as a derived name, so it cannot smuggle in
// markup a member's own message could not.
func TestASummaryIsCleanedLikeAName(t *testing.T) {
	t.Parallel()
	message := &discordgo.Message{Content: "anything"}
	got := threadTitle(t.Context(), titlingClient{reply: "**log house** <@123> pricing!"}, message, "req-1")
	for _, banned := range []string{"*", "<", "@", "!"} {
		if strings.Contains(got, banned) {
			t.Errorf("%q survived into the title %q", banned, got)
		}
	}
}
