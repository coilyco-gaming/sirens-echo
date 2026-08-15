package community

import (
	"context"
	"errors"
	"github.com/bwmarrin/discordgo"
	"net/http"
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

// The reply failing and its notice failing are different member outcomes: one
// got nothing, the other got the answer and no apology. See #675.

// The notice keeps reporting its own failure, which TestAnUndeliverableNotice
// above relies on, and the turn no longer inherits that verdict.
func TestTheNoticeVerdictIsRecordedNotJoined(t *testing.T) {
	t.Parallel()
	rest := &discordgo.RESTError{
		Response: &http.Response{StatusCode: http.StatusForbidden},
		Message:  &discordgo.APIErrorMessage{Code: 50013},
	}
	attrs := discordFailureAttrs(rest)
	failure, ok := attrValue(attrs, "discord_failure")
	if !ok || failure.String() != "rest_error" {
		t.Fatalf("the notice failure classifies as %v, want rest_error", failure)
	}
	status, ok := attrValue(attrs, "discord_status")
	if !ok || status.String() != "403" {
		t.Errorf("the notice verdict lost its status: %v", status)
	}
	// A delivered notice classifies as nothing, so the event does not report a
	// failure that did not happen.
	if got := discordFailureAttrs(nil); len(got) != 0 {
		t.Errorf("a delivered notice was classified: %v", got)
	}
}

// The turn's verdict is the send alone, driven through the seam rather than
// asserted about the classifier. Rejoining the two must fail this.
func TestTheTurnVerdictDescribesTheSend(t *testing.T) {
	t.Parallel()
	agent := &Agent{telemetry: telemetryOrNoop(nil)}
	turn := &alwaysRefusingTurn{}
	err := agent.deliverOrReport(context.Background(), turn, "the answer", nothingWithheld)
	if err == nil {
		t.Fatal("a failed send reported success")
	}
	// The notice was attempted, so this is the both-failed case that used to
	// produce one verdict for two outcomes.
	if turn.attempts < 2 {
		t.Fatalf("the notice was not attempted: %d send(s)", turn.attempts)
	}
	// One error, not a join. errors.Join of two would unwrap to a slice.
	var joined interface{ Unwrap() []error }
	if errors.As(err, &joined) {
		t.Errorf("the turn verdict is a join of %d errors, want the send alone",
			len(joined.Unwrap()))
	}
}
