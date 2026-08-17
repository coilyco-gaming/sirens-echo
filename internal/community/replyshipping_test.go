package community

import (
	"strings"
	"testing"
)

// A validator rejection stops discarding a well-formed reply, and the split is
// what the reply would cost a member rather than which rule fired. See #651.

// A style rule is about how an answer reads, so the answer still lands.
func TestAQualityRefusalStillReachesTheMember(t *testing.T) {
	t.Parallel()
	spans, turn := recordedTurn(t, firstPersonAnswer)

	if turn.reply != firstPersonAnswer {
		t.Errorf("reply = %q, want the model's answer", turn.reply)
	}
	validate := endedSpan(spans, "response.validate")
	if validate == nil {
		t.Fatal("no response.validate span, so this test asserts nothing")
	}
	if got := recordedAttribute(validate, "response.check.shipped"); got != replyCheckResponseStyle {
		t.Errorf("response.check.shipped = %q, want %q", got, replyCheckResponseStyle)
	}
	// Shipped is not passed. A reader must be able to tell the two apart.
	if got := recordedAttribute(validate, "response.check"); got != replyCheckResponseStyle {
		t.Errorf("response.check = %q, want the rule the reply broke", got)
	}
}

// A blast-radius rule still gates, which is the boundary the decision kept.
func TestABlastRadiusRefusalStillBlocksTheReply(t *testing.T) {
	t.Parallel()
	_, turn := recordedTurn(t, "The roster is posted in #invented-room each cycle.")

	if strings.Contains(turn.reply, "#invented-room") {
		t.Errorf("an invented channel reached the member: %q", turn.reply)
	}
}

// The default matters more than the list: a rule nobody classified must gate.
func TestAnUnclassifiedCheckFailsClosed(t *testing.T) {
	t.Parallel()
	for _, check := range []string{
		replyCheckParse,
		replyCheckGrounding,
		replyCheckInventedChannel,
		replyCheckSelfAttributed,
		replyCheckIdentifiers,
		replyCheckIdentityClaim,
		"a_rule_added_next_year",
	} {
		if !checkGates(check) {
			t.Errorf("%s does not gate", check)
		}
	}
	for _, check := range []string{replyCheckResponseStyle, replyCheckToolCallMarkup} {
		if checkGates(check) {
			t.Errorf("%s gates, so a quality rule still discards a reply", check)
		}
	}
}
