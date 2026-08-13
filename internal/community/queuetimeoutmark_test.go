package community

import (
	"context"
	"testing"
	"time"
)

// A turn turned away for load carries a mark, the way a denied one does. See
// sirens-echo#476.

// reactingTurn is a turn that can be marked, which is what a Discord message is
// and what the queue-timeout path takes.
type reactingTurn struct {
	recordingReactor

	transport string
	replies   []string
}

func (t *reactingTurn) RequestID() string { return "req-queue" }
func (t *reactingTurn) Requester() string { return "member" }
func (t *reactingTurn) Transport() string { return t.transport }
func (t *reactingTurn) Current() TranscriptEntry {
	return TranscriptEntry{Content: "how much is copper"}
}

func (t *reactingTurn) History(context.Context) ([]TranscriptEntry, error) {
	return nil, nil
}

func (t *reactingTurn) Reply(_ context.Context, content string) error {
	t.replies = append(t.replies, content)
	return nil
}

// NotifyEvery is what makes the notice one per window. Without it every denial
// notifies and the throttled case cannot be reached.
func queueTimeoutAgent() *Agent {
	agent := &Agent{telemetry: telemetryOrNoop(nil)}
	agent.cfg.RateLimit.NotifyEvery = time.Minute
	agent.ensureRuntimeDefaults()
	return agent
}

// The reported defect. The notice arrived and the member's own message stayed
// unmarked, so it read as never processed.
func TestAQueueTimeoutMarksTheMessage(t *testing.T) {
	t.Parallel()
	turn := &reactingTurn{transport: transportDiscord}
	queueTimeoutAgent().replyQueueTimeout(context.Background(), turn, "guild:1")

	if len(turn.applied) != 1 || turn.applied[0] != reactionFailed {
		t.Errorf("applied = %v, want one %q", turn.applied, reactionFailed)
	}
	if len(turn.replies) != 1 {
		t.Errorf("replies = %v, want the busy notice", turn.replies)
	}
}

// The sharper half. The notice is one per window, so a second member inside
// that window gets no notice, and without the mark would get nothing at all.
func TestAThrottledQueueTimeoutStillMarksTheMessage(t *testing.T) {
	t.Parallel()
	agent := queueTimeoutAgent()
	first := &reactingTurn{transport: transportDiscord}
	agent.replyQueueTimeout(context.Background(), first, "guild:1")

	second := &reactingTurn{transport: transportDiscord}
	agent.replyQueueTimeout(context.Background(), second, "guild:1")

	if len(second.replies) != 0 {
		t.Fatalf("the throttle did not suppress the second notice: %v, so this "+
			"case did not arise", second.replies)
	}
	if len(second.applied) != 1 || second.applied[0] != reactionFailed {
		t.Errorf("a throttled member got %v, so the turn left no trace at all",
			second.applied)
	}
}

// The mark is the transport's, not the path's. A caller with no reaction
// surface still gets its notice and must not panic on the way.
func TestAQueueTimeoutOnATransportWithNoReactions(t *testing.T) {
	t.Parallel()
	turn := &httpTurn{requestID: "req-1"}
	queueTimeoutAgent().replyQueueTimeout(context.Background(), turn, "http")
	// Reaching here is the assertion. The HTTP turn implements no reactor.
}

// A denial and a queue timeout are different outcomes and read differently. One
// is a boundary that turned the message away, the other is load.
func TestALoadTimeoutAndABoundaryDenialDoNotShareAMark(t *testing.T) {
	t.Parallel()
	if reactionFailed == reactionRefused {
		t.Fatal("the two outcomes collapsed onto one glyph")
	}
}
