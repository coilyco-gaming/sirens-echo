package community

import (
	"context"
	"errors"
	"log/slog"
	"net/http"
	"testing"

	"github.com/bwmarrin/discordgo"
)

// A delivery failure that records only that it failed cannot be diagnosed, and
// the reply it dropped was already paid for. See issue 292.

func attrValue(attrs []slog.Attr, key string) (slog.Value, bool) {
	for _, attr := range attrs {
		if attr.Key == key {
			return attr.Value, true
		}
	}
	return slog.Value{}, false
}

// The four candidates QA named are all separable by status plus code, which is
// what makes these two fields sufficient rather than a starting point.
func TestARestErrorIsRecordedByStatusAndCode(t *testing.T) {
	t.Parallel()
	rest := &discordgo.RESTError{
		Response: &http.Response{StatusCode: http.StatusTooManyRequests},
		Message:  &discordgo.APIErrorMessage{Code: 20028},
	}
	attrs := discordFailureAttrs(errors.Join(errors.New("send: "), rest))
	status, ok := attrValue(attrs, "discord_status")
	if !ok || status.Int64() != http.StatusTooManyRequests {
		t.Errorf("status = %v, want 429", status)
	}
	code, ok := attrValue(attrs, "discord_code")
	if !ok || code.Int64() != 20028 {
		t.Errorf("code = %v, want 20028", code)
	}
}

// A gateway that dropped produces no HTTP exchange at all, and that absence is
// a classification rather than a gap.
func TestAFailureWithNoResponseIsItsOwnClass(t *testing.T) {
	t.Parallel()
	attrs := discordFailureAttrs(errors.New("websocket: close 1006"))
	failure, ok := attrValue(attrs, "discord_failure")
	if !ok || failure.String() != "no_response" {
		t.Errorf("failure = %v, want no_response", failure)
	}
	if _, present := attrValue(attrs, "discord_status"); present {
		t.Error("a failure with no response reported a status")
	}
}

// A REST error missing either half must still record the half it has, since a
// partial classification beats none.
func TestAPartialRestErrorStillRecordsWhatItHas(t *testing.T) {
	t.Parallel()
	attrs := discordFailureAttrs(&discordgo.RESTError{
		Response: &http.Response{StatusCode: http.StatusForbidden},
	})
	status, ok := attrValue(attrs, "discord_status")
	if !ok || status.Int64() != http.StatusForbidden {
		t.Errorf("status = %v, want 403", status)
	}
	if _, present := attrValue(attrs, "discord_code"); present {
		t.Error("a REST error with no message reported a code")
	}
}

func TestNoErrorRecordsNothing(t *testing.T) {
	t.Parallel()
	if attrs := discordFailureAttrs(nil); len(attrs) != 0 {
		t.Errorf("a nil error produced %d attributes", len(attrs))
	}
}

// A timeout and a backend outage both fail at the model stage, so error_type
// collapses them. The cause is what an alert can actually separate.
func TestATimeoutAndAnOutageDoNotShareACause(t *testing.T) {
	t.Parallel()
	timeout := failureCause(context.DeadlineExceeded)
	outage := failureCause(errors.New("proxy returned 503"))
	if timeout == outage {
		t.Fatalf("both classified as %q", timeout)
	}
	if timeout != causeTimeout {
		t.Errorf("timeout cause = %q, want %q", timeout, causeTimeout)
	}
}

// The label and the phrase a member reads are picked by the same order, so a
// notice can never describe a condition the telemetry classified differently.
func TestTheCauseAgreesWithTheNoticeAMemberReads(t *testing.T) {
	t.Parallel()
	for _, probe := range []struct {
		cause  error
		notice string
		want   string
	}{
		{context.DeadlineExceeded, noticeTimedOut, causeTimeout},
		{ErrToolRoundsExhausted, noticeRoundsSpent, causeRoundsSpent},
		{errors.New("backend refused"), noticeModelFailed, causeStage},
	} {
		if got := failureCause(probe.cause); got != probe.want {
			t.Errorf("cause = %q, want %q", got, probe.want)
		}
		if got := turnFailureNotice(stageModel, probe.cause); got != probe.notice {
			t.Errorf("notice = %q, want %q", got, probe.notice)
		}
	}
}
