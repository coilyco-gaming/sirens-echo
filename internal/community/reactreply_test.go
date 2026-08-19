package community

import (
	"context"
	"errors"
	"strings"
	"testing"
)

// markableTurn is what a Discord message is: a turn the harness can mark as
// well as reply to.
type markableTurn struct {
	recordingReactor

	replies []string
}

func (t *markableTurn) RequestID() string { return "req-react" }
func (t *markableTurn) Requester() string { return "member" }
func (t *markableTurn) Transport() string { return transportDiscord }
func (t *markableTurn) Current() TranscriptEntry {
	return TranscriptEntry{Author: "member", Content: "thanks, that worked"}
}

func (t *markableTurn) History(context.Context) ([]TranscriptEntry, error) { return nil, nil }

func (t *markableTurn) Reply(_ context.Context, content string) error {
	t.replies = append(t.replies, content)
	return nil
}

// marked reports whether one glyph reached the transport. The turn-accepted
// mark lands before the model call, so the list is never empty.
func marked(turn *markableTurn, glyph string) bool {
	for _, applied := range turn.applied {
		if applied == glyph {
			return true
		}
	}
	return false
}

// toolCallingClient answers with an invocation after calling a tool, which is
// the turn that owes a receipt a mark cannot carry.
type toolCallingClient struct{ reply string }

func (c toolCallingClient) Complete(
	context.Context, TurnPrompt, string,
) (CompletionResult, error) {
	return CompletionResult{
		Content: c.reply,
		ToolCalls: []ExecutedTool{{
			Name: "eco__get_market", Result: "copper 12", Outcome: ToolOutcomeOK,
		}},
	}, nil
}

// A turn whose whole answer is a mark posts nothing and marks the member's own
// message instead.
func TestAReactionInvocationMarksInsteadOfReplying(t *testing.T) {
	t.Parallel()
	agent := silentTurnAgent(answeringClient{reply: "{{react:agree}}"})
	turn := &markableTurn{}

	if err := agent.runTurn(context.Background(), turn, nil); err != nil {
		t.Fatalf("runTurn: %v", err)
	}
	if len(turn.replies) != 0 {
		t.Errorf("the harness posted %v, and the answer was the mark", turn.replies)
	}
	if !marked(turn, replyReactions["agree"]) {
		t.Errorf("applied = %v, want the answer %q among them",
			turn.applied, replyReactions["agree"])
	}
}

// A mark is an answer, so it needs no tool call to have earned its silence the
// way an empty reply does.
func TestAReactionIsNotUnchosenSilence(t *testing.T) {
	t.Parallel()
	for _, key := range reactKeys() {
		agent := silentTurnAgent(answeringClient{reply: "{{react:" + key + "}}"})
		turn := &markableTurn{}
		if err := agent.runTurn(context.Background(), turn, nil); err != nil {
			t.Fatalf("runTurn for %q: %v", key, err)
		}
		if !marked(turn, replyReactions[key]) {
			t.Errorf("key %q applied %v, want %q among them",
				key, turn.applied, replyReactions[key])
		}
	}
}

// A transport with no reaction surface still owes the member the answer, so the
// glyph the invocation stood for is what it sends.
func TestATransportThatCannotMarkSendsTheGlyph(t *testing.T) {
	t.Parallel()
	agent := silentTurnAgent(answeringClient{reply: "{{react:acknowledge}}"})
	turn := &httpTurn{requestID: "react-http", current: TranscriptEntry{
		Author: "member", Content: "noted?",
	}}

	if err := agent.runTurn(context.Background(), turn, nil); err != nil {
		t.Fatalf("runTurn: %v", err)
	}
	if turn.reply != replyReactions["acknowledge"] {
		t.Errorf("reply = %q, want the glyph %q", turn.reply, replyReactions["acknowledge"])
	}
}

// A receipt outranks a mark, because a mark cannot carry one. A turn that called
// a tool replies in words so the footer survives.
func TestATurnOwingAReceiptRepliesRatherThanMarks(t *testing.T) {
	t.Parallel()
	agent := silentTurnAgent(toolCallingClient{reply: "{{react:agree}}"})
	turn := &markableTurn{}

	if err := agent.runTurn(context.Background(), turn, nil); err != nil {
		t.Fatalf("runTurn: %v", err)
	}
	if marked(turn, replyReactions["agree"]) {
		t.Errorf("applied = %v, and the turn owed a receipt a mark cannot carry", turn.applied)
	}
	if len(turn.replies) != 1 {
		t.Fatalf("replies = %v, want one carrying the glyph and the footer", turn.replies)
	}
	if !strings.Contains(turn.replies[0], replyReactions["agree"]) {
		t.Errorf("reply %q dropped the glyph the invocation stood for", turn.replies[0])
	}
	if !strings.Contains(turn.replies[0], "get_market") {
		t.Errorf("reply %q dropped the tool receipt", turn.replies[0])
	}
}

// The likeliest failure is a missing ADD_REACTIONS permission, which must not
// cost the member the answer.
func TestAMarkThatCannotLandIsSentAsWords(t *testing.T) {
	t.Parallel()
	agent := silentTurnAgent(answeringClient{reply: "{{react:disagree}}"})
	turn := &markableTurn{}
	turn.err = errors.New("missing permissions")

	if err := agent.runTurn(context.Background(), turn, nil); err != nil {
		t.Fatalf("runTurn: %v", err)
	}
	if len(turn.replies) != 1 || turn.replies[0] != replyReactions["disagree"] {
		t.Errorf("replies = %v, want the glyph sent as the reply", turn.replies)
	}
}

// An invocation is the whole reply or it is prose with a marker in it, and a
// marker a member reads is the failure this refuses.
func TestAReactionBesideOtherTextIsRefused(t *testing.T) {
	t.Parallel()
	for _, reply := range []string{
		"sure {{react:agree}}",
		"{{react:agree}} {{react:disagree}}",
		"{{react:shrug}}",
	} {
		agent := silentTurnAgent(answeringClient{reply: reply})
		turn := &markableTurn{}
		err := agent.runTurn(context.Background(), turn, nil)
		if err == nil {
			t.Errorf("reply %q was accepted, and a member would read the marker", reply)
		}
		for key, glyph := range replyReactions {
			if marked(turn, glyph) {
				t.Errorf("reply %q resolved to %q anyway", reply, key)
			}
		}
	}
}

// An ordinary reply carries no invocation and must reach the member untouched.
func TestAnOrdinaryReplyIsUnaffected(t *testing.T) {
	t.Parallel()
	agent := silentTurnAgent(answeringClient{reply: "Copper is 12 per unit."})
	turn := &markableTurn{}

	if err := agent.runTurn(context.Background(), turn, nil); err != nil {
		t.Fatalf("runTurn: %v", err)
	}
	if len(turn.replies) != 1 || turn.replies[0] != "Copper is 12 per unit." {
		t.Errorf("replies = %v, want the answer unchanged", turn.replies)
	}
}

// An answer that looks like a progress mark is unreadable, so the two alphabets
// stay disjoint by test rather than by care.
func TestReplyReactionsAreDisjointFromTheHarnessMarks(t *testing.T) {
	t.Parallel()
	marks := map[string]string{
		reactionAccepted: "reactionAccepted",
		reactionTool:     "reactionTool",
		reactionFailed:   "reactionFailed",
		reactionRefused:  "reactionRefused",
	}
	for key, glyph := range replyReactions {
		if name, clash := marks[glyph]; clash {
			t.Errorf("reply reaction %q emits %q, which is also %s. A member "+
				"cannot tell the answer from the progress mark", key, glyph, name)
		}
	}
}

// The prompt is the only thing that tells the model these exist, and a key it
// is not told about is a key it never invokes.
func TestTheReactionPolicyNamesEveryKey(t *testing.T) {
	t.Parallel()
	policy := withReactionPolicy("base prompt")
	if !strings.HasPrefix(policy, "base prompt") {
		t.Fatal("the policy must be appended rather than replace the prompt")
	}
	for _, key := range reactKeys() {
		if !strings.Contains(policy, key) {
			t.Errorf("the prompt never names %q, so the model cannot invoke it", key)
		}
	}
}

// An eval scoring a prompt the service does not render measures nothing, which
// is why the phrase policy was wired into this path in the first place.
func TestTheEvaluationPromptCarriesTheReactionPolicy(t *testing.T) {
	t.Setenv("SIRENS_ECHO_PHRASES", "")
	definition := Definition{Identity: "Sirens Echo", ResponseStyle: ResponseStyleNeutral}
	built, err := evaluationSystemPrompt(definition, PlaceholderPrincipal, "", "policy")
	if err != nil {
		t.Fatalf("evaluationSystemPrompt: %v", err)
	}
	if !strings.Contains(built, "{{react:key}}") {
		t.Error("the eval prompt never names the reaction form the deployed one carries")
	}
}
