package community

import (
	"fmt"

	"github.com/bwmarrin/discordgo"
)

// A long thread was answered from its own tail. See
// docs/sirens-echo-thread-prefill.md.

// prefillNote records what a whole-thread read left out. Zero dropped is the
// ordinary case and renders nothing.
type prefillNote struct {
	Dropped int
	Read    int
	// Capped marks a thread longer than the runtime read, so Read is a floor on
	// the thread's length rather than the length itself.
	Capped bool
}

// truncated covers the walk bound too. A thread whose newest messages fit the
// budget is still incomplete when the read never reached its start.
func (n prefillNote) truncated() bool { return n.Dropped > 0 || n.Capped }

// render states the loss in the reply. A wrong answer from missing context is
// indistinguishable from any other wrong answer without it.
func (n prefillNote) render() string {
	if !n.truncated() {
		return ""
	}
	if n.Dropped == 0 {
		return fmt.Sprintf(
			"[thread truncated: the read stopped at %d messages and did not reach the start]",
			n.Read,
		)
	}
	read := fmt.Sprintf("%d", n.Read)
	if n.Capped {
		read = "at least " + read
	}
	return fmt.Sprintf(
		"[thread truncated: oldest %d of %s messages dropped to fit the context budget]",
		n.Dropped, read,
	)
}

// appendPrefillNoteWithin adds the note the way the tool receipt is added: the
// answer yields, because a note that vanishes under load reads as no loss.
func appendPrefillNoteWithin(reply string, limit int, note prefillNote) string {
	rendered := note.render()
	if rendered == "" {
		return reply
	}
	return appendServiceLine(reply, limit, rendered)
}

// threadPrefillOn reports whether this parent channel opted in. An empty list
// is the shipped default and reads as off for every channel.
func threadPrefillOn(opted []string, parentChannelID string) bool {
	if parentChannelID == "" {
		return false
	}
	for _, id := range opted {
		if id == parentChannelID {
			return true
		}
	}
	return false
}

// messageReader is the half of a Discord session a thread read needs, so the
// walk is testable without one.
type messageReader interface {
	ChannelMessages(
		channelID string, limit int, beforeID, afterID, aroundID string,
		options ...discordgo.RequestOption,
	) ([]*discordgo.Message, error)
}

// readTurnHistory returns a turn's messages oldest first. The whole thread when
// the channel opted in, and the partial window otherwise.
func readTurnHistory(
	reader messageReader, wholeThread bool, channelID, beforeID string, limit int,
) ([]*discordgo.Message, bool, error) {
	if wholeThread {
		return readWholeThread(reader, channelID, beforeID)
	}
	messages, err := reader.ChannelMessages(channelID, limit, beforeID, "", "")
	if err != nil {
		return nil, false, err
	}
	return oldestFirst(messages), false, nil
}

// readWholeThread walks a thread newest first until it runs out or the read
// bound stops it, and returns the messages oldest first.
func readWholeThread(
	reader messageReader, threadID, beforeID string,
) (messages []*discordgo.Message, capped bool, err error) {
	collected := make([]*discordgo.Message, 0, threadPrefillPage)
	cursor := beforeID
	for read := 0; read < threadPrefillReads; read++ {
		page, err := reader.ChannelMessages(threadID, threadPrefillPage, cursor, "", "")
		if err != nil {
			return nil, false, err
		}
		collected = append(collected, page...)
		if len(page) < threadPrefillPage {
			return oldestFirst(collected), false, nil
		}
		cursor = page[len(page)-1].ID
	}
	// The walk stopped rather than the thread ending, so the count that reaches
	// the note is a floor.
	return oldestFirst(collected), true, nil
}

func oldestFirst(messages []*discordgo.Message) []*discordgo.Message {
	reversed := make([]*discordgo.Message, 0, len(messages))
	for index := len(messages) - 1; index >= 0; index-- {
		reversed = append(reversed, messages[index])
	}
	return reversed
}

// dropOldestToFit keeps the newest messages that fit the context budget, which
// is the overflow rule Kai chose over silence, fallback, and summarising.
func dropOldestToFit(entries []TranscriptEntry, budget int) (kept []TranscriptEntry, dropped int) {
	total := 0
	first := len(entries)
	for index := len(entries) - 1; index >= 0; index-- {
		total += len(entries[index].Content)
		if total > budget {
			break
		}
		first = index
	}
	return entries[first:], first
}
