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
	if got := stageLine(stagePhraseThinking); got != want {
		t.Errorf("thinking notice = %q, want %q", got, want)
	}
}

// Only thinking and the clock lines are terminated. Kai settled that on
// sirens-echo#370 after the ellipsis had been applied to all four stages.
func TestOnlyThinkingCarriesTheEllipsis(t *testing.T) {
	t.Parallel()
	for phrase, want := range map[string]string{
		stagePhraseHistory:  "> `reading recent messages`",
		stagePhraseTool:     "> `calling a tool`",
		stagePhraseChecking: "> `checking the reply`",
	} {
		if got := stageLine(phrase); got != want {
			t.Errorf("stage %q = %q, want %q", phrase, got, want)
		}
	}
	if got := stageLine(stagePhraseHistory); strings.Contains(got, "  ") {
		t.Error("an empty icon left a double space")
	}
}

// An icon named later must not bring an ellipsis nobody asked for, which is
// why the two are separate fields rather than one implying the other.
func TestAnIconDoesNotImplyAnEllipsis(t *testing.T) {
	t.Parallel()
	got := stageNotice("\U0001F528", stagePhraseTool, false)
	if got != "> \U0001F528 `calling a tool`" {
		t.Errorf("decorated tool notice = %q", got)
	}
}

func TestEveryStageStillMatchesTheNoticeShape(t *testing.T) {
	t.Parallel()
	for _, phrase := range []string{
		stagePhraseHistory, stagePhraseThinking, stagePhraseTool, stagePhraseSkill,
		stagePhraseChecking,
	} {
		notice := stageLine(phrase)
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
	got := stageNotice("\U0001F914", "Thinking: about <@123> & things!", true)
	if !noticeShape.MatchString(got) {
		t.Fatalf("sanitized notice escapes the shape: %q", got)
	}
	for _, banned := range []string{"@", "&", "!", ":", "<"} {
		if strings.Contains(got, banned) {
			t.Errorf("%q survived sanitizing into %q", banned, got)
		}
	}
}

func TestSkillRoundNarratesTheCatalogue(t *testing.T) {
	if got := toolStagePhrase(skillToolServer); got != stagePhraseSkill {
		t.Fatalf("skill round phrase = %q", got)
	}
	if got := toolStagePhrase("eco"); got != stagePhraseTool {
		t.Fatalf("ordinary round phrase = %q", got)
	}
	rendered := stageLine(stagePhraseSkill)
	want := "> " + skillReadGlyph + " `consulting the catalogue...`"
	if rendered != want {
		t.Fatalf("stage line = %q, want %q", rendered, want)
	}
	if !noticeShape.MatchString(rendered) {
		t.Fatalf("stage line %q escapes the notice shape", rendered)
	}
}
