package community

import (
	"fmt"
	"strings"
	"testing"
)

// Removing the block rather than the message, for the reply in
// sirens-echo#796. The fixtures are in replyrepair_test.go.

// redactor is the checks as the runtime wires them, with no identity so the
// grounding rules are the only ones in play.
func redactor() *Agent {
	return &Agent{
		cfg:         Config{Definition: Definition{}},
		telemetry:   telemetryOrNoop(nil),
		identifiers: NewIdentifierGuard(Config{}, nil),
	}
}

// redact runs the refusal and the removal together, the way the turn does, so
// a test never has to name the rule the checks would have named.
func redact(t *testing.T, reply string) (string, int, bool) {
	t.Helper()
	agent := redactor()
	_, rule, err := agent.runReplyChecks(reply, TurnPrompt{}, CompletionResult{})
	if err == nil {
		t.Fatalf("the reply was accepted, so there is nothing to redact:\n%s", reply)
	}
	return agent.redactRefusedBlocks(reply, rule, TurnPrompt{}, CompletionResult{})
}

// The ask: eleven correct blocks are not lost to the twelfth.
func TestTheOffendingBlockGoesAndTheRestSurvives(t *testing.T) {
	t.Parallel()
	kept, blocks, ok := redact(t, groundedBlocks+inventedBlock)

	if !ok {
		t.Fatal("the reply was refused whole, so nothing was saved")
	}
	if blocks != 1 {
		t.Errorf("removed %d blocks, want 1", blocks)
	}
	if !strings.Contains(kept, "notifications/initialized: Bad Request") {
		t.Errorf("the outage report went with the invented channel:\n%s", kept)
	}
	if strings.Contains(kept, "#general") {
		t.Errorf("the invented channel survived the redaction:\n%s", kept)
	}
}

// A hole a member cannot see is worse than a refusal, so the removal is marked
// in the one shape model prose cannot forge.
func TestTheRemovalIsMarkedInTheHarnessShape(t *testing.T) {
	t.Parallel()
	kept, _, ok := redact(t, groundedBlocks+inventedBlock)

	if !ok {
		t.Fatal("the reply was refused whole, so there is no mark to read")
	}
	if !strings.Contains(kept, noticeRedacted) {
		t.Errorf("the redaction is unmarked:\n%s", kept)
	}
	marked := false
	for _, line := range strings.Split(kept, "\n") {
		if line == noticeRedacted {
			marked = noticeShape.MatchString(line)
		}
	}
	if !marked {
		t.Errorf("the mark is not a harness notice, so it reads as model prose:\n%s", kept)
	}
}

// The mark replaces the block it removed, so the reader sees where the gap is
// rather than a message that reads as complete.
func TestTheMarkStandsWhereTheBlockStood(t *testing.T) {
	t.Parallel()
	kept, _, ok := redact(t, groundedBlocks+inventedBlock)

	if !ok {
		t.Fatal("the reply was refused whole")
	}
	lines := strings.Split(kept, "\n")
	if len(lines) != 4 {
		t.Fatalf("want four lines, three kept and one mark, got %d:\n%s", len(lines), kept)
	}
	if lines[3] != noticeRedacted {
		t.Errorf("the mark is not where the removed block was:\n%s", kept)
	}
}

// The safety net. A block failing a different rule is not removed, so the
// second pass is what stops it being delivered.
func TestTheRemainderMustPassEveryCheckOnItsOwn(t *testing.T) {
	t.Parallel()
	_, _, ok := redact(t,
		groundedBlocks+inventedBlock+
			"\n- eco - answered, the service is now monitoring the queue.")

	if ok {
		t.Error("a remainder that still fails a check was delivered")
	}
}

// Two blocks carrying the same rule both go, because the cap is what bounds
// the removal and neither block is the message.
func TestTwoBlocksOnTheSameRuleBothGo(t *testing.T) {
	t.Parallel()
	kept, blocks, ok := redact(t,
		groundedBlocks+inventedBlock+
			"\n- eco - answered, the notice is pinned in #announcements.")

	if !ok {
		t.Fatal("two removable blocks were refused whole")
	}
	if blocks != 2 {
		t.Errorf("removed %d blocks, want 2", blocks)
	}
	if strings.Contains(kept, "#general") || strings.Contains(kept, "#announcements") {
		t.Errorf("an invented channel survived:\n%s", kept)
	}
}

// Past the cap this is not a message with a bad block in it, and the member is
// better served by the refusal than by a page of marks.
func TestRedactionStopsAtTheCap(t *testing.T) {
	t.Parallel()
	bad := ""
	for over := 0; over < maxRedactedBlocks+1; over++ {
		bad += fmt.Sprintf("\n- server - answered, posted in #channel%d.", over)
	}

	if _, _, ok := redact(t, groundedBlocks+bad); ok {
		t.Errorf("more than %d blocks were removed", maxRedactedBlocks)
	}
}

// A rule about the whole reply cannot be answered by removing part of it, and
// identifier disclosure reads across block boundaries.
func TestAWholeReplyRuleIsRefusedWhole(t *testing.T) {
	t.Parallel()
	agent := redactor()
	for _, rule := range []string{
		replyCheckParse,
		replyCheckIdentifiers,
		replyCheckIdentityClaim,
		replyCheckResponseStyle,
	} {
		t.Run(rule, func(t *testing.T) {
			t.Parallel()
			_, _, ok := agent.redactRefusedBlocks(
				groundedBlocks+inventedBlock, rule, TurnPrompt{}, CompletionResult{},
			)
			if ok {
				t.Errorf("%s was answered by removing a block", rule)
			}
		})
	}
}

// One block is the whole message. Removing it and marking the hole delivers
// nothing but the mark.
func TestASingleBlockReplyIsRefusedWhole(t *testing.T) {
	t.Parallel()
	if _, _, ok := redact(t, "The roster is posted in #invented-room each cycle."); ok {
		t.Error("a one-block reply was redacted down to its own mark")
	}
}

// End to end, because every assertion above is about a function rather than
// about what a member gets. The turn is the thing sirens-echo#796 measured.
func TestAMemberReceivesTheRedactedReply(t *testing.T) {
	t.Parallel()
	spans, turn := recordedTurn(t, groundedBlocks+inventedBlock)

	if !strings.Contains(turn.reply, "notifications/initialized: Bad Request") {
		t.Errorf("the member got no outage report:\n%s", turn.reply)
	}
	if strings.Contains(turn.reply, "#general") {
		t.Errorf("the member got the invented channel:\n%s", turn.reply)
	}
	if !strings.Contains(turn.reply, noticeRedacted) {
		t.Errorf("the member got an unmarked hole:\n%s", turn.reply)
	}
	validate := endedSpan(spans, "response.validate")
	if validate == nil {
		t.Fatal("no response.validate span, so this test asserts nothing")
	}
	if got := recordedAttribute(validate, "response.check"); got != replyCheckInventedChannel {
		t.Errorf("response.check = %q, want %q", got, replyCheckInventedChannel)
	}
}

// A reply nothing refused says so, so a redaction is never read into a clean
// turn. The count is on every turn rather than only the redacted ones.
func TestACleanReplyRecordsNoRedaction(t *testing.T) {
	t.Parallel()
	spans, _ := recordedTurn(t, "Twelve rows are listed.")

	validate := endedSpan(spans, "response.validate")
	if validate == nil {
		t.Fatal("no response.validate span, so this test asserts nothing")
	}
	if got := recordedAttribute(validate, "response.redacted.blocks"); got != "0" {
		t.Errorf("response.redacted.blocks = %q, want 0 on a clean turn", got)
	}
}

// Splitting has to be lossless, or a kept block reaches the member reshaped.
func TestKeepingEveryBlockRebuildsTheReply(t *testing.T) {
	t.Parallel()
	for name, reply := range map[string]string{
		"bulleted":  groundedBlocks + inventedBlock,
		"prose":     "First paragraph, two lines.\nStill the first.\n\nSecond paragraph.",
		"numbered":  "1. one\n2. two\n3. three",
		"mixed":     "Lead line.\n\n- one\n- two\n\nClosing line.",
		"one block": "A single sentence.",
	} {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			blocks := splitReplyBlocks(reply)
			keep := make([]bool, len(blocks))
			for index := range keep {
				keep[index] = true
			}
			if got := keptText(blocks, keep); got != reply {
				t.Errorf("round trip changed the reply:\n%q\nwant:\n%q", got, reply)
			}
		})
	}
}
