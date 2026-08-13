package community

import (
	"context"
	"errors"
	"strings"
	"testing"
)

// A reply that was composed and did not arrive left the member with silence,
// which is indistinguishable from being ignored. See issue 137.

// recordingTurn accepts the notice and records it. The reply that failed did so
// before this path is reached, so the notice send is the first call here.
type refusingTurn struct {
	sent []string
}

func (t *refusingTurn) RequestID() string        { return "undelivered" }
func (t *refusingTurn) Requester() string        { return "318190481467244544" }
func (t *refusingTurn) Transport() string        { return transportDiscord }
func (t *refusingTurn) Current() TranscriptEntry { return TranscriptEntry{} }
func (t *refusingTurn) History(context.Context) ([]TranscriptEntry, error) {
	return nil, nil
}

func (t *refusingTurn) Reply(_ context.Context, content string) error {
	t.sent = append(t.sent, content)
	return nil
}

func TestAnUndeliveredReplyTellsTheMember(t *testing.T) {
	t.Parallel()
	turn := &refusingTurn{}
	agent := &Agent{telemetry: telemetryOrNoop(nil)}
	if err := agent.reportUndelivered(context.Background(), turn); err != nil {
		t.Fatalf("reportUndelivered: %v", err)
	}
	if len(turn.sent) != 1 {
		t.Fatalf("expected one notice, got %d: %v", len(turn.sent), turn.sent)
	}
	if !strings.Contains(turn.sent[0], "could not be delivered") {
		t.Errorf("the notice does not name delivery: %q", turn.sent[0])
	}
}

// One attempt, never a retry. A loop here turns one dropped reply into a flood
// against the same transport that just refused.
func TestAnUndeliverableNoticeIsNotRetried(t *testing.T) {
	t.Parallel()
	turn := &alwaysRefusingTurn{}
	agent := &Agent{telemetry: telemetryOrNoop(nil)}
	if err := agent.reportUndelivered(context.Background(), turn); err == nil {
		t.Fatal("a notice that could not be sent reported success")
	}
	if turn.attempts != 1 {
		t.Errorf("the notice was attempted %d times, want 1", turn.attempts)
	}
}

type alwaysRefusingTurn struct{ attempts int }

func (t *alwaysRefusingTurn) RequestID() string        { return "undeliverable" }
func (t *alwaysRefusingTurn) Requester() string        { return "318190481467244544" }
func (t *alwaysRefusingTurn) Transport() string        { return transportDiscord }
func (t *alwaysRefusingTurn) Current() TranscriptEntry { return TranscriptEntry{} }
func (t *alwaysRefusingTurn) History(context.Context) ([]TranscriptEntry, error) {
	return nil, nil
}

func (t *alwaysRefusingTurn) Reply(context.Context, string) error {
	t.attempts++
	return errors.New("still refused")
}
