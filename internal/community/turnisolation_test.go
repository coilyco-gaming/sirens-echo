package community

import (
	"strings"
	"testing"
)

// A turn's prompt must carry nothing from the turn before it. See
// sirens-echo#265 and docs/sirens-echo-turn-stages.md.

// The bleed a later change would most plausibly introduce is an accumulator or
// a cache, added for a good reason, that survives one call into the next.
func TestASecondTurnCarriesNothingFromTheFirst(t *testing.T) {
	t.Parallel()
	first := BuildTurnPrompt(
		BuildSystemPrompt(
			Definition{Identity: "Sirens Deep of Coilyco", AuditRole: "general"},
			Principal{Handle: "first_handle", UserID: "1111111111111111111"},
			"FIRST-BUNDLE-SENTINEL",
			"FIRST-POLICY-SENTINEL",
		),
		[]TranscriptEntry{{Author: "first_author", Content: "FIRST-HISTORY-SENTINEL"}},
		TranscriptEntry{Author: "first_author", Content: "FIRST-CURRENT-SENTINEL"},
	)
	second := BuildTurnPrompt(
		BuildSystemPrompt(
			Definition{Identity: "Sirens Deep of Coilyco", AuditRole: "general"},
			Principal{Handle: "second_handle", UserID: "2222222222222222222"},
			"SECOND-BUNDLE-SENTINEL",
			"SECOND-POLICY-SENTINEL",
		),
		[]TranscriptEntry{{Author: "second_author", Content: "SECOND-HISTORY-SENTINEL"}},
		TranscriptEntry{Author: "second_author", Content: "SECOND-CURRENT-SENTINEL"},
	)
	whole := second.System + "\n" + second.Context + "\n" + second.Message
	for _, leaked := range []string{
		"FIRST-BUNDLE-SENTINEL",
		"FIRST-POLICY-SENTINEL",
		"FIRST-HISTORY-SENTINEL",
		"FIRST-CURRENT-SENTINEL",
		"first_author",
		"first_handle",
		"1111111111111111111",
	} {
		if strings.Contains(whole, leaked) {
			t.Errorf("the second turn carries %s from the first. Prompt assembly is "+
				"holding state between turns, which is how a context bleed arrives",
				leaked)
		}
	}
	// The control. A test that only checks for absence passes on an empty
	// prompt, which is absence for the wrong reason.
	if !strings.Contains(whole, "SECOND-HISTORY-SENTINEL") ||
		!strings.Contains(first.System, "FIRST-BUNDLE-SENTINEL") {
		t.Fatal("a turn did not carry its own inputs, so the absence above proves nothing")
	}
}

// Assembly reads its arguments and nothing else, so the same inputs produce the
// same prompt however many turns ran in between.
func TestRepeatingATurnProducesTheSamePrompt(t *testing.T) {
	t.Parallel()
	build := func(marker string) TurnPrompt {
		return BuildTurnPrompt(
			BuildSystemPrompt(
				Definition{Identity: "Sirens Echo of Coilyco", AuditRole: "general"},
				PlaceholderPrincipal, "", "a local policy root",
			),
			[]TranscriptEntry{{Author: "member", Content: marker}},
			TranscriptEntry{Author: "member", Content: "the current message"},
		)
	}
	before := build("run-one")
	build("an unrelated turn in between")
	after := build("run-one")
	if before.System != after.System {
		t.Error("the same inputs produced two different system prompts, so assembly " +
			"depends on something other than its arguments")
	}
	if before.Context != after.Context || before.Message != after.Message {
		t.Error("the same inputs produced two different turn contexts")
	}
}

// The caller's history is the only conversation content a turn reads. See
// docs/sirens-echo-turn-stages.md for what that does and does not bound.
func TestTheTurnContextHoldsOnlyTheSuppliedEntries(t *testing.T) {
	t.Parallel()
	supplied := []TranscriptEntry{
		{Author: "member", Content: "the first supplied line"},
		{Author: "another member", Content: "the second supplied line"},
	}
	prompt := BuildTurnPrompt("a system prompt", supplied,
		TranscriptEntry{Author: "member", Content: "the current message"})
	lines := 0
	for _, line := range strings.Split(prompt.Context, "\n") {
		if strings.HasPrefix(strings.TrimSpace(line), "-") {
			lines++
		}
	}
	if lines != len(supplied) {
		t.Errorf("the turn context holds %d transcript lines for %d supplied entries. "+
			"An entry nobody supplied is content from outside this turn",
			lines, len(supplied))
	}
}
