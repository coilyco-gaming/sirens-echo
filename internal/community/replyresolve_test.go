package community

import (
	"strings"
	"testing"

	"github.com/bwmarrin/discordgo"
)

// Discord does not always deliver the referenced message inline, and it is
// least likely to for the old ones. See sirens-echo#630.

// The headline. A reference the Gateway truncated still reaches the model, so
// the reply this feature exists for is the one it now serves.
func TestATruncatedReferenceStillNamesTheSubject(t *testing.T) {
	t.Parallel()
	message := &discordgo.Message{
		Author:           &discordgo.User{Username: "bob"},
		Content:          "is that still true",
		MessageReference: &discordgo.MessageReference{MessageID: "9001"},
	}
	resolved := &discordgo.Message{
		Author:  &discordgo.User{Username: "alice"},
		Content: "the plank market crashed",
	}

	subject := replyTarget(message, resolved)

	if subject == nil {
		t.Fatal("a truncated reference resolved to nothing")
	}
	if subject.Author != "alice" || subject.Content != "the plank market crashed" {
		t.Errorf("subject = %+v, want the resolved message", subject)
	}
}

// The inline payload still wins, so a fetched message can never contradict what
// Discord already delivered.
func TestTheInlineReferenceOutranksAResolvedOne(t *testing.T) {
	t.Parallel()
	message := &discordgo.Message{
		Author:            &discordgo.User{Username: "bob"},
		ReferencedMessage: &discordgo.Message{Author: &discordgo.User{Username: "alice"}},
	}
	other := &discordgo.Message{Author: &discordgo.User{Username: "carol"}}

	if got := replyTarget(message, other); got.Author != "alice" {
		t.Errorf("subject author = %q, want the inline alice", got.Author)
	}
}

// A message replying to nothing gains no subject from either source, which is
// every ordinary message in the channel.
func TestAMessageThatRepliesToNothingHasNoSubject(t *testing.T) {
	t.Parallel()
	message := &discordgo.Message{Author: &discordgo.User{Username: "bob"}}

	if got := replyTarget(message, nil); got != nil {
		t.Errorf("subject = %+v, want none", got)
	}
	if got := replyTarget(nil, nil); got != nil {
		t.Errorf("a nil message produced %+v", got)
	}
}

// Replying to a screenshot used to render as replying to empty text, which is
// the defect the attachment suffix exists for.
func TestAReplyToAnImageSaysThereWasAnImage(t *testing.T) {
	t.Parallel()
	current := TranscriptEntry{
		Author:  "bob",
		Content: "what does this say",
		ReplyTo: &ReplySubject{
			Author:      "alice",
			Attachments: []string{"image/png"},
		},
	}

	context := buildTurnContext(nil, current)

	if !strings.Contains(context, "cannot read: image/png") {
		t.Errorf("the replied-to attachment was not reported:\n%s", context)
	}
}

// A reply to a message with no attachment renders exactly as before, so the
// suffix costs nothing on the ordinary path.
func TestAReplyWithNoAttachmentGainsNoSuffix(t *testing.T) {
	t.Parallel()
	current := TranscriptEntry{
		Author:  "bob",
		Content: "is that still true",
		ReplyTo: &ReplySubject{Author: "alice", Content: "the market crashed"},
	}

	context := buildTurnContext(nil, current)

	if strings.Contains(context, "cannot read") {
		t.Errorf("a text reply gained an attachment suffix:\n%s", context)
	}
	if !strings.Contains(context, "bob is replying to alice: the market crashed") {
		t.Errorf("the reply line changed shape:\n%s", context)
	}
}

// Discord delivers the reference inline for most replies, and resolving one
// must not cost a REST call when it did.
func TestAnInlineReferenceCostsNoLookup(t *testing.T) {
	t.Parallel()
	agent := &Agent{telemetry: telemetryOrNoop(nil)}
	agent.ensureRuntimeDefaults()
	message := &discordgo.Message{
		Author:            &discordgo.User{Username: "bob"},
		ReferencedMessage: &discordgo.Message{Author: &discordgo.User{Username: "alice"}},
		MessageReference:  &discordgo.MessageReference{MessageID: "9001"},
	}

	// A nil session would panic on any REST call, which is what proves there
	// was none. The inline message is used directly by replyTarget.
	if got := agent.resolveReplyTo(nil, message, summonContext{}); got != nil {
		t.Error("an inline reference was fetched anyway")
	}
}

// A message carrying no reference resolves to nothing without reaching the
// network, which is every ordinary message.
func TestAMessageWithNoReferenceReachesNoNetwork(t *testing.T) {
	t.Parallel()
	agent := &Agent{telemetry: telemetryOrNoop(nil)}
	agent.ensureRuntimeDefaults()

	for _, message := range []*discordgo.Message{
		{Author: &discordgo.User{Username: "bob"}},
		{
			Author:           &discordgo.User{Username: "bob"},
			MessageReference: &discordgo.MessageReference{},
		},
	} {
		if got := agent.resolveReplyTo(nil, message, summonContext{}); got != nil {
			t.Errorf("%+v resolved to a message", message.MessageReference)
		}
	}
}
