package community

import (
	"strings"
	"testing"
)

// A reply names its subject, and the channel's latest message is not it.
// See sirens-echo#579.

func replyingTurn() TranscriptEntry {
	return TranscriptEntry{
		Author:  "alpha",
		Content: "is this still true?",
		ReplyTo: &ReplySubject{
			Author:  "coilysiren",
			Content: "the wipe is scheduled for Tuesday",
		},
	}
}

func TestAReplyNamesWhatItAnswers(t *testing.T) {
	t.Parallel()
	context := buildTurnContext(nil, replyingTurn())
	if !strings.Contains(context, "alpha is replying to coilysiren") {
		t.Errorf("the reply target is not named: %q", context)
	}
	if !strings.Contains(context, "the wipe is scheduled for Tuesday") {
		t.Errorf("the replied-to content is missing: %q", context)
	}
}

// The subject must not depend on how many messages arrived since, which is the
// behaviour the issue reports.
func TestTheReplyTargetSurvivesAFullHistoryWindow(t *testing.T) {
	t.Parallel()
	history := make([]TranscriptEntry, 0, 8)
	for range 8 {
		history = append(history, TranscriptEntry{Author: "beta", Content: "unrelated chatter"})
	}
	context := buildTurnContext(history, replyingTurn())
	if !strings.Contains(context, "alpha is replying to coilysiren") {
		t.Errorf("a busy channel lost the reply target: %q", context)
	}
	// The request line still arrives, so the reply line was inserted rather
	// than substituted for it.
	if !strings.Contains(context, "The request that follows is from alpha") {
		t.Errorf("the request line was displaced: %q", context)
	}
}

// A turn that is not a reply must render byte-identically to before.
func TestANonReplyIsUnchanged(t *testing.T) {
	t.Parallel()
	plain := TranscriptEntry{Author: "alpha", Content: "is this still true?"}
	for name, history := range map[string][]TranscriptEntry{
		"no history":   nil,
		"with history": {{Author: "beta", Content: "hello"}},
	} {
		context := buildTurnContext(history, plain)
		if strings.Contains(context, "is replying to") {
			t.Errorf("%s invented a reply target: %q", name, context)
		}
	}
}

// An empty or partial reference must not produce a line naming nobody.
func TestAnEmptyReferenceRendersNothing(t *testing.T) {
	t.Parallel()
	for name, target := range map[string]*ReplySubject{
		"both empty": {},
		"whitespace": {Author: "  ", Content: "\t"},
	} {
		entry := TranscriptEntry{Author: "alpha", Content: "hi", ReplyTo: target}
		if got := entry.replyLine("alpha"); got != "" {
			t.Errorf("%s produced a reply line: %q", name, got)
		}
	}
	// A reference with content but no author still says what was answered,
	// because the content is the part that carries the subject.
	entry := TranscriptEntry{
		Author:  "alpha",
		Content: "hi",
		ReplyTo: &ReplySubject{Content: "the wipe is Tuesday"},
	}
	if got := entry.replyLine("alpha"); !strings.Contains(got, "the wipe is Tuesday") {
		t.Errorf("an authorless reference lost its content: %q", got)
	}
}
