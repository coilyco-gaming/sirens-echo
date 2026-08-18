package community

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"strings"
	"testing"

	"github.com/bwmarrin/discordgo"
	metricnoop "go.opentelemetry.io/otel/metric/noop"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
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

// The window is the wait plus two beats, whatever the wait is set to. See
// docs/sirens-echo-progress.md and sirens-echo#375.
func TestTheThreadWindowIsTheOneThatWasAskedFor(t *testing.T) {
	t.Parallel()
	if turnLongReplyAfter != turnProgressAfter+turnProgressEvery*2 {
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
	got := threadTitle(t.Context(), titlingClient{reply: "log house pricing"}, message, "req-1", nil)
	if got != "log house pricing" {
		t.Errorf("title = %q, want the summary", got)
	}
}

// A title that fails must not cost a member their thread.
func TestAFailedTitleKeepsTheDerivedName(t *testing.T) {
	t.Parallel()
	message := &discordgo.Message{Content: "how much does it cost to build a log house"}
	if got := threadTitle(t.Context(), titlingClient{fail: true}, message, "req-1", nil); got != "" {
		t.Errorf("a failed title returned %q, want empty so the caller falls back", got)
	}
	if got := threadTitle(t.Context(), nil, message, "req-1", nil); got != "" {
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
	got := threadTitle(t.Context(), titlingClient{reply: "**log house** <@123> pricing!"}, message, "req-1", nil)
	for _, banned := range []string{"*", "<", "@", "!"} {
		if strings.Contains(got, banned) {
			t.Errorf("%q survived into the title %q", banned, got)
		}
	}
}

// A title that comes back over-length is regenerated, not trimmed. See #753.

// statedTitleLimit comes from the knob, so raising the bound cannot strand a
// test on a sentence the code stopped writing. See sirens-echo#904.
func statedTitleLimit() string {
	return fmt.Sprintf("at most %d characters", threadTitleRunes)
}

// lengthAwareClient answers long until the request states the bound, so a test
// can tell a regeneration from a retry of the same prompt.
type lengthAwareClient struct {
	long    string
	short   string
	prompts []string
}

func (c *lengthAwareClient) Complete(
	_ context.Context, prompt TurnPrompt, _ string,
) (CompletionResult, error) {
	c.prompts = append(c.prompts, prompt.System)
	if strings.Contains(prompt.System, statedTitleLimit()) {
		return CompletionResult{Content: c.short}, nil
	}
	return CompletionResult{Content: c.long}, nil
}

// The headline. An over-length title is asked again rather than cut, and the
// second request states the limit.
func TestAnOverLongTitleIsRegeneratedWithTheLimitStated(t *testing.T) {
	t.Parallel()
	client := &lengthAwareClient{
		long:  strings.Repeat("market price comparison ", 5),
		short: "market price comparison",
	}

	got := threadTitle(t.Context(), client, &discordgo.Message{Content: "prices?"}, "req-1", nil)

	if got != "market price comparison" {
		t.Errorf("title = %q, want the regenerated one", got)
	}
	if len(client.prompts) != 2 {
		t.Fatalf("made %d titling calls, want exactly one regeneration", len(client.prompts))
	}
	if strings.Contains(client.prompts[0], statedTitleLimit()) {
		t.Error("the first request already stated the limit, so the second proves nothing")
	}
	if !strings.Contains(client.prompts[1], statedTitleLimit()) {
		t.Errorf("the second request does not state the limit: %q", client.prompts[1])
	}
}

// The budget property. Two over-length answers stop, they do not loop.
func TestTwoOverLongTitlesFallBackToAHardTrim(t *testing.T) {
	t.Parallel()
	var recorded strings.Builder
	telemetry, err := newTelemetry(
		slog.New(slog.NewJSONHandler(&recorded, nil)),
		sdktrace.NewTracerProvider(),
		metricnoop.NewMeterProvider(),
	)
	if err != nil {
		t.Fatalf("telemetry: %v", err)
	}
	overLong := strings.Repeat("market price comparison ", 5)
	client := &lengthAwareClient{long: overLong, short: overLong}

	got := threadTitle(
		t.Context(), client, &discordgo.Message{Content: "prices?"}, "req-1", telemetry)

	if len([]rune(got)) > threadTitleRunes {
		t.Errorf("the fallback is %d runes, over the %d bound", len([]rune(got)), threadTitleRunes)
	}
	if len(client.prompts) != 2 {
		t.Errorf("made %d titling calls, want the retry bounded at one", len(client.prompts))
	}
	// A hard trim, so the bound is not spent saying it was reached.
	if strings.Contains(got, "…") {
		t.Errorf("the fallback trimmed with an ellipsis: %q", got)
	}
	// Silent trimming hides a generator that keeps overrunning.
	if !strings.Contains(recorded.String(), "thread.title.trimmed") {
		t.Errorf("the hard trim recorded nothing:\n%s", recorded.String())
	}
}

// The creation path is where the bound has to hold, because the derived name
// never went through the generator at all.
func TestTheCreationPathBoundsEveryThreadName(t *testing.T) {
	t.Parallel()
	// An over-long generated title, and an absent one so the derived name is
	// what reaches the thread list. Both are names a member would read.
	message := &discordgo.Message{
		Content: strings.Repeat("how much does a log house cost ", 8),
	}
	for _, title := range []string{strings.Repeat("status ", 40), ""} {
		got := threadCreationName(title, message)
		if len([]rune(got)) > threadTitleRunes {
			t.Errorf("a thread name reached %d runes, over the %d bound: %q",
				len([]rune(got)), threadTitleRunes, got)
		}
		if got == "" {
			t.Error("the creation path produced no name, which Discord refuses")
		}
	}
}

// A well-behaved title costs exactly one call, so the retry never becomes the
// ordinary path.
func TestATitleInsideTheBoundIsNotRegenerated(t *testing.T) {
	t.Parallel()
	client := &lengthAwareClient{long: "log house pricing", short: "unused"}

	if got := threadTitle(
		t.Context(), client, &discordgo.Message{Content: "cost?"}, "req-1", nil,
	); got != "log house pricing" {
		t.Errorf("title = %q", got)
	}
	if len(client.prompts) != 1 {
		t.Errorf("made %d titling calls for a title already inside the bound", len(client.prompts))
	}
}

// The reported clip was around 30 characters against a 50-rune bound, because
// the bound was never what decided length. See sirens-echo#904.
func TestTheTitlerIsAskedForTheLengthTheBoundAllows(t *testing.T) {
	t.Parallel()
	client := &lengthAwareClient{long: "market price comparison", short: "market price comparison"}

	threadTitle(t.Context(), client, &discordgo.Message{Content: "prices?"}, "req-1", nil)

	if len(client.prompts) == 0 {
		t.Fatal("no titling call was made, so this test asserts nothing")
	}
	if !strings.Contains(client.prompts[0], fmt.Sprintf("at most %d words", threadTitleWords)) {
		t.Errorf("the titler was not asked for the word budget: %q", client.prompts[0])
	}
	// A word averages about five characters plus its space, so the word budget
	// has to be able to reach the range the issue asked for.
	if reachable := threadTitleWords * 6; reachable < 40 {
		t.Errorf("%d words reaches about %d characters, under the 40 floor",
			threadTitleWords, reachable)
	}
	if threadTitleRunes < 40 || threadTitleRunes > 60 {
		t.Errorf("the title bound is %d, outside the 40 to 60 range", threadTitleRunes)
	}
	// The room has to exist for the words to use, or the retry fires every time.
	if threadTitleWords*6 > threadTitleRunes+6 {
		t.Errorf("%d words overshoots a %d rune bound", threadTitleWords, threadTitleRunes)
	}
}
