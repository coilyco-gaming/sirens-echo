package community

import (
	"context"
	"testing"
)

// #1035: four of Dowel's visible posts in #moxn-temporal carried no content at
// all. A blank message reads as the thing being broken while carrying none of
// the information a real failure would.

func blankReplyAgent() *Agent {
	return &Agent{telemetry: telemetryOrNoop(nil)}
}

func TestABlankReplyIsNeverSent(t *testing.T) {
	t.Parallel()
	for _, content := range []string{"", " ", "\n", "\t\n  ", " "} {
		turn := &markableTurn{}
		if err := blankReplyAgent().sendReply(t.Context(), turn, content, ""); err != nil {
			t.Fatalf("sendReply(%q) returned %v", content, err)
		}
		if len(turn.replies) != 0 {
			t.Errorf("content %q was sent as %q, want nothing sent", content, turn.replies)
		}
	}
}

// The turn still lands somewhere a member can see, which separates this from
// silently dropping the answer.
func TestABlankReplyMarksTheMessageInstead(t *testing.T) {
	t.Parallel()
	turn := &markableTurn{}
	if err := blankReplyAgent().sendReply(t.Context(), turn, "   ", ""); err != nil {
		t.Fatalf("sendReply returned %v", err)
	}
	if !marked(turn, replyReactions[blankReplyReaction]) {
		t.Errorf("marks = %v, want %q", turn.applied, replyReactions[blankReplyReaction])
	}
}

// quietTurn has no reaction surface, and must still not fall back to sending
// the blank it could not mark.
type quietTurn struct{ replies []string }

func (t *quietTurn) RequestID() string                                  { return "req-quiet" }
func (t *quietTurn) Requester() string                                  { return "member" }
func (t *quietTurn) Transport() string                                  { return transportHTTP }
func (t *quietTurn) Current() TranscriptEntry                           { return TranscriptEntry{} }
func (t *quietTurn) History(context.Context) ([]TranscriptEntry, error) { return nil, nil }

func (t *quietTurn) Reply(_ context.Context, content string) error {
	t.replies = append(t.replies, content)
	return nil
}

func TestATransportThatCannotMarkStillSendsNothing(t *testing.T) {
	t.Parallel()
	turn := &quietTurn{}
	if err := blankReplyAgent().sendReply(t.Context(), turn, "", ""); err != nil {
		t.Fatalf("sendReply returned %v", err)
	}
	if len(turn.replies) != 0 {
		t.Errorf("a blank was sent as %q on a transport that cannot mark", turn.replies)
	}
}

// The guard must not swallow real answers, including short ones.
func TestARealReplyIsUnaffected(t *testing.T) {
	t.Parallel()
	turn := &quietTurn{}
	if err := blankReplyAgent().sendReply(t.Context(), turn, "ok", ""); err != nil {
		t.Fatalf("sendReply returned %v", err)
	}
	if len(turn.replies) != 1 || turn.replies[0] != "ok" {
		t.Errorf("replies = %v, want one \"ok\"", turn.replies)
	}
}
