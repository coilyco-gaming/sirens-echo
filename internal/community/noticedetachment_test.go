package community

import (
	"context"
	"testing"
	"time"
)

// notifyFailure detaches twice and its comment names one. See sirens-echo#627.

// deadlineTurn records the context its Reply was handed, so a test can ask what
// survived the detachment rather than inferring it from the send succeeding.
type deadlineTurn struct {
	sent        []string
	cancelled   bool
	deadline    time.Time
	hadDeadline bool
}

func (t *deadlineTurn) RequestID() string        { return "detach" }
func (t *deadlineTurn) Requester() string        { return "318190481467244544" }
func (t *deadlineTurn) Transport() string        { return transportHTTP }
func (t *deadlineTurn) Current() TranscriptEntry { return TranscriptEntry{} }
func (t *deadlineTurn) History(context.Context) ([]TranscriptEntry, error) {
	return nil, nil
}

func (t *deadlineTurn) Reply(ctx context.Context, content string) error {
	t.sent = append(t.sent, content)
	t.cancelled = ctx.Err() != nil
	t.deadline, t.hadDeadline = ctx.Deadline()
	return nil
}

// The expiring turn is the case this protects and the common one. Without
// WithoutCancel the notice would be refused by its own context.
func TestANoticeSendsAfterTheTurnIsCancelled(t *testing.T) {
	t.Parallel()
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	// The control: the turn context really is dead before the notice is built.
	if ctx.Err() == nil {
		t.Fatal("the fixture is not a cancelled turn, so this proves nothing")
	}
	turn := &deadlineTurn{}
	agent := &Agent{telemetry: telemetryOrNoop(nil)}
	if err := agent.notifyFailure(ctx, turn, "> `turn failed`"); err != nil {
		t.Fatalf("notifyFailure on a cancelled turn: %v", err)
	}
	if len(turn.sent) != 1 {
		t.Fatalf("expected one notice, got %d", len(turn.sent))
	}
	if turn.cancelled {
		t.Error("the notice was handed an already-cancelled context")
	}
}

// The second detachment: its own bound rather than none, so a wedged send
// cannot hang the failure path.
func TestTheNoticeCarriesItsOwnDeadline(t *testing.T) {
	t.Parallel()
	turn := &deadlineTurn{}
	agent := &Agent{telemetry: telemetryOrNoop(nil)}
	if err := agent.notifyFailure(context.Background(), turn, "> `turn failed`"); err != nil {
		t.Fatalf("notifyFailure: %v", err)
	}
	if !turn.hadDeadline {
		t.Fatal("the notice ran unbounded, so a wedged send hangs the failure path")
	}
	if remaining := time.Until(turn.deadline); remaining > failureNoticeTimeout {
		t.Errorf("notice deadline is %s away, longer than failureNoticeTimeout %s",
			remaining, failureNoticeTimeout)
	}
}

// A turn whose own deadline is longer must not lend it to the notice, or the
// bound above is decorative.
func TestALongTurnDeadlineDoesNotBecomeTheNoticesDeadline(t *testing.T) {
	t.Parallel()
	ctx, cancel := context.WithTimeout(context.Background(), time.Hour)
	defer cancel()
	turn := &deadlineTurn{}
	agent := &Agent{telemetry: telemetryOrNoop(nil)}
	if err := agent.notifyFailure(ctx, turn, "> `turn failed`"); err != nil {
		t.Fatalf("notifyFailure: %v", err)
	}
	if remaining := time.Until(turn.deadline); remaining > failureNoticeTimeout {
		t.Errorf("the notice inherited the turn's hour: %s remaining", remaining)
	}
}
