package community

import (
	"context"
	"fmt"
	"log/slog"
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
func threadTitlePrompt() string {
	return fmt.Sprintf(
		"Name this request in at most %d words, as a topic rather than a sentence. "+
			"No punctuation at the end, no quotes, no preamble. Reply with the name "+
			"and nothing else.",
		threadTitleWords,
	)
}

// threadTitleRetryPrompt states the bound the first attempt missed. Built from
// the knob, so the sentence cannot drift away from the number.
func threadTitleRetryPrompt() string {
	return fmt.Sprintf(
		"%s The name must be at most %d characters.",
		threadTitlePrompt(),
		threadTitleRunes,
	)
}

// threadTitle summarises what the member asked for. A failure returns empty and
// the caller keeps the derived name. See docs/sirens-echo-threads.md.
func threadTitle(
	ctx context.Context,
	completions CompletionClient,
	message *discordgo.Message,
	requestID string,
	telemetry *Telemetry,
) string {
	if completions == nil || message == nil {
		return ""
	}
	name := threadTitleAttempt(ctx, completions, message, requestID, threadTitlePrompt())
	if withinTitleBound(name) {
		return name
	}
	// Regenerated rather than trimmed: a cut title loses its subject, which is
	// the one thing a title carries. See docs/sirens-echo-threads.md.
	retried := threadTitleAttempt(
		ctx, completions, message, requestID, threadTitleRetryPrompt())
	if withinTitleBound(retried) && retried != "" {
		return retried
	}
	// One regeneration, never a loop. A title generator must not be able to
	// spend a turn's budget on itself.
	trimmed := hardTrimRunes(valueOrDefault(retried, name), threadTitleRunes)
	telemetryOrNoop(telemetry).Info(
		ctx,
		"thread.title.trimmed",
		slog.Int("attempts", 2),
		slog.Int("limit_runes", threadTitleRunes),
		slog.Int("trimmed_runes", len([]rune(trimmed))),
	)
	return trimmed
}

// threadTitleAttempt runs one titling call. An error returns empty, because no
// failure here is worth failing a turn over.
func threadTitleAttempt(
	ctx context.Context,
	completions CompletionClient,
	message *discordgo.Message,
	requestID string,
	system string,
) string {
	result, err := completions.Complete(ctx, TurnPrompt{
		System:  system,
		Message: message.ContentWithMentionsReplaced(),
	}, requestID)
	if err != nil {
		return ""
	}
	// The same cleaning the derived name takes, so a summary cannot smuggle in
	// markup a member's message could not.
	return threadNameFrom(result.Content)
}

// withinTitleBound also accepts empty, which means the call failed and the
// caller falls back to the derived name rather than regenerating.
func withinTitleBound(name string) bool {
	return len([]rune(name)) <= threadTitleRunes
}

// hardTrimRunes cuts without an ellipsis. truncateRunes spends a rune saying it
// truncated, which is the thing this bound exists to avoid.
func hardTrimRunes(value string, limit int) string {
	runes := []rune(value)
	if len(runes) <= limit {
		return value
	}
	return strings.TrimSpace(string(runes[:limit]))
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

// threadCreationName is the name a thread is created with, bounded whatever it
// came from. See docs/sirens-echo-threads.md.
func threadCreationName(title string, message *discordgo.Message) string {
	return hardTrimRunes(
		valueOrDefault(title, threadNameFor(message)), threadTitleRunes)
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
			Name:                threadCreationName(title, message),
			AutoArchiveDuration: threadArchiveMinutes,
			Invitable:           false,
		},
	)
	if err != nil || thread == nil || thread.ID == "" {
		return "", false
	}
	return thread.ID, true
}
