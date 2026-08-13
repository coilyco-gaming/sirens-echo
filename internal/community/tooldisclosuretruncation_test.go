package community

import (
	"strings"
	"testing"
)

// Characterization. A service-authored suffix is appended before the Discord
// send budget is applied, so a long reply loses it. See sirens-echo#385.
func TestALongReplyLosesItsDisclosureFooterToTruncation(t *testing.T) {
	t.Parallel()
	answer := strings.Repeat("a", discordReplyLimit)
	full := AppendToolDisclosure(answer, ExecutedTool{
		Name:    "eco.get_market",
		Outcome: ToolOutcomeFailed,
	})
	if !strings.Contains(full, "❌") {
		t.Fatalf("the footer was not appended at all: %q", full[len(full)-40:])
	}

	sent := truncateRunes(full, discordReplyLimit)
	if strings.Contains(sent, "❌") {
		t.Error("the footer now survives truncation, so sirens-echo#385's " +
			"transport budget landed and this assertion should follow it")
	}
	// The reason it matters: absence reads as no tools ran, which is the one
	// thing the footer exists to make impossible to believe.
	if strings.Contains(sent, "🔨") {
		t.Error("a partial footer survived, which is worse than none")
	}
}

// The same hazard predates the footer. Issue references are appended in the
// same position and nobody has hit it yet. See sirens-echo#385.
func TestAnIssueReferenceIsTruncatableForTheSameReason(t *testing.T) {
	t.Parallel()
	answer := strings.Repeat("b", discordReplyLimit)
	full := AppendToolDisclosure(answer, ExecutedTool{
		Name:    "forgejo.create_issue",
		Outcome: ToolOutcomeOK,
	})
	if len([]rune(full)) <= discordReplyLimit {
		t.Fatal("the fixture no longer exceeds the budget, so it proves nothing")
	}
	if len([]rune(truncateRunes(full, discordReplyLimit))) != discordReplyLimit {
		t.Error("truncation no longer cuts to the budget")
	}
}
