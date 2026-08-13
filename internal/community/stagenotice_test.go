package community

import (
	"strings"
	"testing"
)

// A progress line should say that it is still going, not just what it is.
// See sirens-echo#370.

func TestTheThinkingLineIsWhatWasAskedFor(t *testing.T) {
	t.Parallel()
	want := "> \U0001F914 `thinking...`"
	if got := stageNotice(stageIcons[stagePhraseThinking], stagePhraseThinking); got != want {
		t.Errorf("thinking notice = %q, want %q", got, want)
	}
}

// A stage with no icon renders the plain shape, so the three stages nobody has
// chosen an icon for are unchanged apart from the ellipsis.
func TestAnUndecoratedStageKeepsThePlainShape(t *testing.T) {
	t.Parallel()
	got := stageNotice(stageIcons[stagePhraseHistory], stagePhraseHistory)
	if got != "> `reading recent messages...`" {
		t.Errorf("history notice = %q", got)
	}
	if strings.Contains(got, "  ") {
		t.Error("an empty icon left a double space")
	}
}

func TestEveryStageStillMatchesTheNoticeShape(t *testing.T) {
	t.Parallel()
	for _, phrase := range []string{
		stagePhraseHistory, stagePhraseThinking, stagePhraseTool, stagePhraseChecking,
	} {
		notice := stageNotice(stageIcons[phrase], phrase)
		if !noticeShape.MatchString(notice) {
			t.Errorf("stage %q renders outside the notice shape: %q", phrase, notice)
		}
	}
}

// The shape is what lets a member tell a harness line from model output, so
// widening it for an icon must not widen it for prose.
func TestOnlyANonASCIIIconCanLeadANotice(t *testing.T) {
	t.Parallel()
	for _, posing := range []string{
		"> Sure! `thinking...`",
		"> I think `thinking...`",
		"> <@1494729988799336548> `thinking...`",
		"> ** `thinking...`",
	} {
		if noticeShape.MatchString(posing) {
			t.Errorf("ASCII prose passed as a harness line: %q", posing)
		}
	}
	// The plain and decorated forms both still pass, which is the point.
	for _, real := range []string{"> `turn failed`", "> \U0001F914 `thinking...`"} {
		if !noticeShape.MatchString(real) {
			t.Errorf("a real notice was rejected: %q", real)
		}
	}
}

// The body still takes the alphabet. An icon decorates the line; it does not
// license anything inside the phrase.
func TestTheDecoratedBodyIsStillSanitized(t *testing.T) {
	t.Parallel()
	got := stageNotice("\U0001F914", "Thinking: about <@123> & things!")
	if !noticeShape.MatchString(got) {
		t.Fatalf("sanitized notice escapes the shape: %q", got)
	}
	for _, banned := range []string{"@", "&", "!", ":", "<"} {
		if strings.Contains(got, banned) {
			t.Errorf("%q survived sanitizing into %q", banned, got)
		}
	}
}
