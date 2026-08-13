package community

import (
	"strings"
	"unicode"

	"github.com/bwmarrin/discordgo"
)

// A long answer gets somewhere of its own. The channel keeps the progress line,
// which is what points at it. See docs/sirens-echo-threads.md.

// threadArchiveMinutes matches the guild's own hide-after setting, so a thread
// does not outlive the channel's expectation of it.
const threadArchiveMinutes = 60

// threadNameRunes is Discord's cap. A longer name is refused outright.
const threadNameRunes = 100

// threadNameFallback names a thread whose message was all mention and
// punctuation, so a thread is never created without a readable name.
const threadNameFallback = "a longer answer"

// threadNameFor derives a name from the member's own message rather than
// authoring one. Mentions and markup are dropped, not summarised.
func threadNameFor(message *discordgo.Message) string {
	if message == nil {
		return threadNameFallback
	}
	cleaned := strings.Map(func(r rune) rune {
		if unicode.IsLetter(r) || unicode.IsDigit(r) || unicode.IsSpace(r) {
			return r
		}
		return ' '
	}, message.ContentWithMentionsReplaced())
	name := strings.Join(strings.Fields(cleaned), " ")
	if name == "" {
		return threadNameFallback
	}
	return truncateRunes(name, threadNameRunes)
}

// threadForReply returns where a long turn's reply lands. No failure here is
// worth failing a turn over. See docs/sirens-echo-threads.md.
func threadForReply(
	session *discordgo.Session,
	message *discordgo.Message,
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
			Name:                threadNameFor(message),
			AutoArchiveDuration: threadArchiveMinutes,
			Invitable:           false,
		},
	)
	if err != nil || thread == nil || thread.ID == "" {
		return "", false
	}
	return thread.ID, true
}
