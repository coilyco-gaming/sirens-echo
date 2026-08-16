package community

import (
	"fmt"
	"strings"
	"testing"

	"github.com/bwmarrin/discordgo"
)

// A thread longer than the window was answered from its own tail, and nothing
// said so. See issue 769.

// pagedReader serves a thread newest first, the way Discord does.
type pagedReader struct {
	messages []*discordgo.Message
	calls    int
}

func threadOf(count int, body string) *pagedReader {
	messages := make([]*discordgo.Message, 0, count)
	// Newest first, so index 0 is the most recent, matching Discord.
	for index := count; index >= 1; index-- {
		messages = append(messages, &discordgo.Message{
			ID:      fmt.Sprintf("%06d", index),
			Content: fmt.Sprintf("m%d %s", index, body),
			Author:  &discordgo.User{Username: "member"},
		})
	}
	return &pagedReader{messages: messages}
}

func (r *pagedReader) ChannelMessages(
	_ string, limit int, beforeID, _, _ string,
	_ ...discordgo.RequestOption,
) ([]*discordgo.Message, error) {
	r.calls++
	start := 0
	if beforeID != "" {
		for index, message := range r.messages {
			if message.ID == beforeID {
				start = index + 1
				break
			}
		}
	}
	if start >= len(r.messages) {
		return nil, nil
	}
	end := start + limit
	if end > len(r.messages) {
		end = len(r.messages)
	}
	return r.messages[start:end], nil
}

// The whole point: a thread longer than one page comes back whole and in order.
func TestAWholeThreadIsReadPastOnePage(t *testing.T) {
	t.Parallel()
	reader := threadOf(250, "hello")
	messages, capped, err := readWholeThread(reader, "thread-1", "")
	if err != nil {
		t.Fatalf("readWholeThread: %v", err)
	}
	if capped {
		t.Error("a 250 message thread reported as capped")
	}
	if len(messages) != 250 {
		t.Fatalf("read %d messages, want 250", len(messages))
	}
	// Oldest first, which is the order the prompt is built in.
	if !strings.HasPrefix(messages[0].Content, "m1 ") {
		t.Errorf("first message is %q, want the oldest", messages[0].Content)
	}
	if !strings.HasPrefix(messages[249].Content, "m250 ") {
		t.Errorf("last message is %q, want the newest", messages[249].Content)
	}
	if reader.calls != 3 {
		t.Errorf("read the thread in %d calls, want 3", reader.calls)
	}
}

// The walk is bounded, so a pathological thread costs a known number of calls.
// Capped marks the count as a floor rather than the thread's length.
func TestAThreadLongerThanTheWalkReportsCapped(t *testing.T) {
	t.Parallel()
	reader := threadOf(threadPrefillPage*threadPrefillReads+50, "hello")
	messages, capped, err := readWholeThread(reader, "thread-1", "")
	if err != nil {
		t.Fatalf("readWholeThread: %v", err)
	}
	if !capped {
		t.Error("a thread longer than the walk did not report capped")
	}
	if reader.calls != threadPrefillReads {
		t.Errorf("made %d calls, want the bound of %d", reader.calls, threadPrefillReads)
	}
	if len(messages) != threadPrefillPage*threadPrefillReads {
		t.Errorf("read %d messages, want %d", len(messages), threadPrefillPage*threadPrefillReads)
	}
}

// Kai chose dropping oldest first over silence, fallback, and summarising.
func TestOverflowDropsTheOldestMessagesFirst(t *testing.T) {
	t.Parallel()
	entries := make([]TranscriptEntry, 0, 10)
	for index := 1; index <= 10; index++ {
		entries = append(entries, TranscriptEntry{Content: strings.Repeat("x", 100)})
	}
	// Same 100 bytes as the rest, so the budget arithmetic stays obvious.
	entries[9].Content = strings.Repeat("x", 94) + "newest"
	kept, dropped := dropOldestToFit(entries, 450)
	if dropped != 6 {
		t.Errorf("dropped %d, want 6", dropped)
	}
	if len(kept) != 4 {
		t.Fatalf("kept %d, want 4", len(kept))
	}
	if !strings.HasSuffix(kept[len(kept)-1].Content, "newest") {
		t.Error("the newest message was dropped")
	}
}

// Nothing over budget drops nothing, which is every ordinary thread.
func TestAThreadInsideTheBudgetDropsNothing(t *testing.T) {
	t.Parallel()
	entries := []TranscriptEntry{{Content: "one"}, {Content: "two"}}
	kept, dropped := dropOldestToFit(entries, threadPrefillBytes)
	if dropped != 0 || len(kept) != 2 {
		t.Errorf("dropped %d and kept %d, want 0 and 2", dropped, len(kept))
	}
}

// The acceptance: the annotation is present whenever any message was dropped,
// and it names how many.
func TestTheAnnotationIsPresentWheneverAMessageWasDropped(t *testing.T) {
	t.Parallel()
	for _, note := range []prefillNote{
		{Dropped: 1, Read: 2},
		{Dropped: 47, Read: 210},
		{Dropped: 900, Read: 1000, Capped: true},
	} {
		sent := AssembleReply("The answer.", discordReplyLimit)
		withNote := assembleReplyFacts(
			"The answer.", discordReplyLimit, serviceFacts{prefill: note},
		)
		if withNote == sent {
			t.Fatalf("%d dropped produced no annotation", note.Dropped)
		}
		if !strings.Contains(withNote, fmt.Sprintf("oldest %d of", note.Dropped)) {
			t.Errorf("the annotation does not name %d dropped: %q", note.Dropped, withNote)
		}
	}
}

// A capped read knows a floor rather than a length, and says the floor.
func TestACappedReadSaysItsCountIsAFloor(t *testing.T) {
	t.Parallel()
	rendered := prefillNote{Dropped: 900, Read: 1000, Capped: true}.render()
	if !strings.Contains(rendered, "at least 1000") {
		t.Errorf("a capped read claimed an exact length: %q", rendered)
	}
	exact := prefillNote{Dropped: 47, Read: 210}.render()
	if strings.Contains(exact, "at least") {
		t.Errorf("an exhausted read hedged its length: %q", exact)
	}
}

// Nothing dropped renders nothing, so an ordinary reply is byte-identical.
func TestNoTruncationAddsNothingToTheReply(t *testing.T) {
	t.Parallel()
	answer := "The Saturday build starts in the afternoon."
	if got := assembleReplyFacts(answer, discordReplyLimit, serviceFacts{}); got != answer {
		t.Errorf("an untruncated turn changed the reply: %q", got)
	}
	if got := (prefillNote{Read: 12}).render(); got != "" {
		t.Errorf("a complete read rendered %q", got)
	}
}

// Outside a thread nothing changes, which is the ordinary window read in the
// ordinary order.
func TestOutsideAThreadTheWindowIsUnchanged(t *testing.T) {
	t.Parallel()
	reader := threadOf(5, "hello")
	turn := &discordMessageTurn{
		message: &discordgo.Message{ID: "999999", ChannelID: "c-1"},
		limit:   3,
	}
	messages, capped, err := readTurnHistory(
		reader, turn.wholeThread, turn.message.ChannelID, turn.message.ID, turn.limit,
	)
	if err != nil {
		t.Fatalf("readHistory: %v", err)
	}
	if capped {
		t.Error("a window read reported capped")
	}
	if len(messages) != 3 {
		t.Fatalf("read %d messages, want the window of 3", len(messages))
	}
	if reader.calls != 1 {
		t.Errorf("a window read cost %d calls, want 1", reader.calls)
	}
	if !strings.HasPrefix(messages[0].Content, "m3 ") {
		t.Errorf("first message is %q, want the oldest of the window", messages[0].Content)
	}
	if turn.PrefillNote().Read != 0 {
		t.Error("a window read produced a prefill note")
	}
}

// A read that fails is the turn's failure, not a silent empty history.
func TestAFailedThreadReadIsReported(t *testing.T) {
	t.Parallel()
	_, _, err := readWholeThread(failingReader{}, "thread-1", "")
	if err == nil {
		t.Fatal("a failed read reported success")
	}
}

type failingReader struct{}

func (failingReader) ChannelMessages(
	string, int, string, string, string, ...discordgo.RequestOption,
) ([]*discordgo.Message, error) {
	return nil, fmt.Errorf("discord refused")
}

// discordEnv sets the minimum a Discord deployment needs to load, so a thread
// prefill test states only what it is about.
func discordEnv(t *testing.T, channels string) {
	t.Helper()
	t.Setenv("SIRENS_ECHO_DEFINITION", definitionOf(t, "deep"))
	useFixtureBundles(t, "creator")
	t.Setenv("SIRENS_ECHO_STEAM_MCP_URL", "http://sirens-deep-steam-mcp:9112/mcp")
	t.Setenv("SIRENS_ECHO_FORGEJO_MCP_URL", "http://sirens-deep-forgejo-mcp:8080/mcp")
	t.Setenv("SIRENS_ECHO_DISCORD_ENABLED", "true")
	t.Setenv("SIRENS_ECHO_INSTANCE", "sirens-deep")
	t.Setenv("DISCORD_TOKEN", "discord-token")
	t.Setenv("DISCORD_CHANNEL_ID", channels)
	t.Setenv("AGENT_PROXY_MODEL", "model")
}

// Every thread reads whole, with nothing to configure. The per-channel toggle
// was removed on sirens-echo#769: this is not a per-channel thing.
func TestEveryThreadReadsWholeWithNothingConfigured(t *testing.T) {
	t.Parallel()
	reader := threadOf(250, "hello")
	inside := &discordMessageTurn{
		message:     &discordgo.Message{ID: "999999", ChannelID: "thread-1"},
		limit:       3,
		wholeThread: true,
	}

	messages, capped, err := readTurnHistory(
		reader, inside.wholeThread,
		inside.message.ChannelID, inside.message.ID, inside.limit,
	)

	if err != nil {
		t.Fatalf("readTurnHistory: %v", err)
	}
	if capped {
		t.Error("a 250-message thread reported capped")
	}
	// The window limit of 3 must not bound a thread read, which is the whole
	// point of the feature.
	if len(messages) != 250 {
		t.Errorf("read %d messages, want the whole thread", len(messages))
	}
}

// A deployment may still carry the retired env var. It is inert now, and a
// stale value must not fail boot for a service that no longer reads it.
func TestTheRetiredToggleEnvVarIsInert(t *testing.T) {
	discordEnv(t, "1024000000000000001")
	t.Setenv("SIRENS_ECHO_THREAD_PREFILL_CHANNELS", "1024000000000000009")

	if _, err := LoadConfig(); err != nil {
		t.Errorf("a stale thread-prefill toggle failed boot: %v", err)
	}
}

// A thread whose newest messages all fit the budget is still incomplete when
// the walk never reached its start, and that has to be said too.
func TestACappedWalkIsAnnotatedEvenWhenNothingWentOverBudget(t *testing.T) {
	t.Parallel()
	note := prefillNote{Dropped: 0, Read: threadPrefillPage * threadPrefillReads, Capped: true}
	if !note.truncated() {
		t.Fatal("a capped walk with nothing over budget read as complete")
	}
	rendered := note.render()
	if !strings.Contains(rendered, "did not reach the start") {
		t.Errorf("the annotation does not say the read stopped short: %q", rendered)
	}
	if strings.Contains(rendered, "dropped to fit") {
		t.Errorf("the annotation blamed the budget for a walk bound: %q", rendered)
	}
}
