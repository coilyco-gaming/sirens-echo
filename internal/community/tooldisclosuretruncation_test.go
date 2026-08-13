package community

import (
	"strings"
	"testing"
)

// Flipped from a characterization test. The footer used to be appended before
// the send budget applied, so a long reply lost it. See sirens-echo#385.
func TestALongReplyKeepsItsDisclosureFooter(t *testing.T) {
	t.Parallel()
	answer := strings.Repeat("a", discordReplyLimit)
	sent := AppendToolDisclosureWithin(answer, discordReplyLimit, ExecutedTool{
		Name:    "eco.get_market",
		Outcome: ToolOutcomeFailed,
	})

	if got := len([]rune(sent)); got > discordReplyLimit {
		t.Errorf("reply is %d runes, over the %d budget", got, discordReplyLimit)
	}
	// Absence reads as no tools ran, which is the one thing the footer exists
	// to make impossible to believe.
	if !strings.Contains(sent, "❌") || !strings.Contains(sent, "`eco.get_market`") {
		t.Error("the footer was truncated away, so a failed call became invisible")
	}
	if !strings.HasPrefix(sent, "aaa") {
		t.Error("the answer was discarded rather than shortened")
	}
}

// The answer yields only as much as the footer needs, so an ordinary reply is
// untouched by the budget.
func TestAShortReplyIsNotShortenedByTheBudget(t *testing.T) {
	t.Parallel()
	unbounded := AppendToolDisclosure("Copper is 2.4c.", ran("eco.get_market", ToolOutcomeOK))
	bounded := AppendToolDisclosureWithin(
		"Copper is 2.4c.", discordReplyLimit, ran("eco.get_market", ToolOutcomeOK),
	)
	if bounded != unbounded {
		t.Errorf("budget changed a reply that already fits:\n%q\nvs\n%q", bounded, unbounded)
	}
}

// A transport with no ceiling declares none, and the HTTP turn is that case.
func TestAnUnboundedTransportGetsNoBudget(t *testing.T) {
	t.Parallel()
	if got := replyLimitOf(&httpTurn{requestID: "r-1"}); got != 0 {
		t.Errorf("HTTP reply limit = %d, want 0", got)
	}
	if got := replyLimitOf(&discordMessageTurn{}); got != discordReplyLimit {
		t.Errorf("Discord reply limit = %d, want %d", got, discordReplyLimit)
	}
}

// A footer alone over budget is the degenerate case. It must not produce a
// reply that is only blank lines.
func TestAFooterLargerThanTheBudgetStillReturnsTheFooter(t *testing.T) {
	t.Parallel()
	sent := AppendToolDisclosureWithin("some answer", 4, ran("eco.get_market", ToolOutcomeOK))
	if strings.TrimSpace(sent) == "" {
		t.Fatal("a tiny budget produced an empty reply")
	}
	if !strings.Contains(sent, "`eco.get_market`") {
		t.Errorf("reply = %q, want the footer", sent)
	}
}

// A reference is a link a member can act on and the footer is a receipt. This
// asserted on footerBudget, which was correct while the send was not.
func TestAReferenceIsNeverShortenedForTheFooter(t *testing.T) {
	t.Parallel()
	answer := strings.Repeat("a", discordReplyLimit)
	sent := AssembleReply(answer, discordReplyLimit, createIssueCall())

	if got := len([]rune(sent)); got > discordReplyLimit {
		t.Errorf("reply is %d runes, over the %d budget", got, discordReplyLimit)
	}
	if !strings.Contains(sent, wantIssue233) {
		t.Errorf("the link a member can act on was lost:\n%q", sent)
	}
	// No reference to protect means the receipt is the only suffix, and the
	// answer still yields to it rather than the other way round.
	noReference := AssembleReply(
		answer, discordReplyLimit, ran("eco.get_market", ToolOutcomeOK),
	)
	if !strings.Contains(noReference, "`eco.get_market`") {
		t.Errorf("the receipt was truncated away:\n%q", noReference)
	}
}
