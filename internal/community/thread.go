package community

import (
	"context"
	"strings"
	"unicode"

	"github.com/bwmarrin/discordgo"
)

// A long answer gets somewhere of its own. The channel keeps the progress line,
// which is what points at it. See docs/sirens-echo-threads.md.

// threadNameFallback names a thread whose message was all mention and
// punctuation, so a thread is never created without a readable name.
const threadNameFallback = "a longer answer"

// threadTitlePrompt asks for the intent rather than a restatement. Kai's
// example: how much does it cost to build a log house, to log house pricing.
const threadTitlePrompt = "Name this request in at most six words, as a topic " +
	"rather than a sentence. No punctuation at the end, no quotes, no preamble. " +
	"Reply with the name and nothing else."

// threadTitle summarises what the member asked for. A failure returns empty and
// the caller keeps the derived name. See docs/sirens-echo-threads.md.
func threadTitle(
	ctx context.Context,
	completions CompletionClient,
	message *discordgo.Message,
	requestID string,
) string {
	if completions == nil || message == nil {
		return ""
	}
	result, err := completions.Complete(ctx, TurnPrompt{
		System:  threadTitlePrompt,
		Message: message.ContentWithMentionsReplaced(),
	}, requestID)
	if err != nil {
		return ""
	}
	// The same cleaning the derived name takes, so a summary cannot smuggle in
	// markup a member's message could not.
	return threadNameFrom(result.Content)
}

// threadNameFor derives a name from the member's own message rather than
// authoring one. Mentions and markup are dropped, not summarised.
func threadNameFor(message *discordgo.Message) string {
	if message == nil {
		return threadNameFallback
	}
	if name := threadNameFrom(message.ContentWithMentionsReplaced()); name != "" {
		return name
	}
	return threadNameFallback
}

// threadNameFrom keeps letters, digits and spaces and bounds the result. Empty
// means nothing usable survived.
func threadNameFrom(raw string) string {
	cleaned := strings.Map(func(r rune) rune {
		if unicode.IsLetter(r) || unicode.IsDigit(r) || unicode.IsSpace(r) {
			return r
		}
		return ' '
	}, raw)
	name := strings.Join(strings.Fields(cleaned), " ")
	if name == "" {
		return ""
	}
	return truncateRunes(name, threadNameRunes)
}

// threadForReply returns where a long turn's reply lands. No failure here is
// worth failing a turn over. See docs/sirens-echo-threads.md.
func threadForReply(
	session *discordgo.Session,
	message *discordgo.Message,
	title string,
) (channelID string, threaded bool) {
	if session == nil || message == nil {
		return "", false
	}
	// A turn already inside a thread has one. Nesting is not permitted by
	// Discord and would be the wrong shape even if it were.
	if session.State != nil {
		channel, err := session.State.Channel(message.ChannelID)
		if err == nil && channel != nil && channel.IsThread() {
			return "", false
		}
	}
	thread, err := session.MessageThreadStartComplex(
		message.ChannelID,
		message.ID,
		&discordgo.ThreadStart{
			Name:                valueOrDefault(title, threadNameFor(message)),
			AutoArchiveDuration: threadArchiveMinutes,
			Invitable:           false,
		},
	)
	if err != nil || thread == nil || thread.ID == "" {
		return "", false
	}
	return thread.ID, true
}
