package community

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"strings"
	"testing"
)

// A round that returned HTTP 200 is never a backend availability failure: the
// answer being unusable is a fact about the answer. See sirens-echo#933.

func TestAnOversizedAnswerIsNotReportedAsAnOutage(t *testing.T) {
	t.Parallel()
	// The shape both read paths produce: the bound, wrapped.
	cause := fmt.Errorf("%w of %d bytes", ErrResponseTooLarge, 2*1024*1024)

	notice := turnFailureNotice(stageModel, cause)
	if notice == noticeModelFailed {
		t.Fatalf("a 200 whose body was too long still reports as %q, which "+
			"sends a member to retry against a healthy backend", notice)
	}
	if notice != noticeResponseTooLarge {
		t.Errorf("notice = %q", notice)
	}
	if got := failureCause(cause); got != causeResponseTooLarge {
		t.Errorf("failure cause = %q, want %q", got, causeResponseTooLarge)
	}
	if !noticeShape.MatchString(notice) {
		t.Errorf("%q is not a harness notice", notice)
	}
}

func TestAnUnreadableAnswerIsNotReportedAsAnOutage(t *testing.T) {
	t.Parallel()
	cause := fmt.Errorf("%w: decode: %w", ErrResponseUnreadable, errors.New("invalid character"))

	notice := turnFailureNotice(stageModel, cause)
	if notice == noticeModelFailed {
		t.Fatalf("a 200 with an unparsable body still reports as %q", notice)
	}
	if notice != noticeResponseUnreadable {
		t.Errorf("notice = %q", notice)
	}
	if got := failureCause(cause); got != causeResponseUnreadable {
		t.Errorf("failure cause = %q, want %q", got, causeResponseUnreadable)
	}
}

// The other half of the acceptance: a genuine outage keeps a string that means
// only that, so the two stay tellable apart in a channel and in telemetry.
func TestAGenuineBackendFailureStillSaysUnavailable(t *testing.T) {
	t.Parallel()
	transport := fmt.Errorf("Agent Proxy request: %w", errors.New("connection refused"))
	if got := turnFailureNotice(stageModel, transport); got != noticeModelFailed {
		t.Fatalf("an unreachable backend reports as %q, want the outage notice", got)
	}
	if got := failureCause(transport); got != causeStage {
		t.Errorf("failure cause = %q, want %q", got, causeStage)
	}
	// A 5xx is the backend answering that it cannot serve, which is the one
	// case the availability string is for.
	unavailable := modelHTTPError{Status: http.StatusBadGateway}
	if got := turnFailureNotice(stageModel, unavailable); got != noticeModelFailed {
		t.Fatalf("a 502 reports as %q, want the outage notice", got)
	}
}

// A slow answer was already classified correctly, and this change must not
// move it: the deadline case sits ahead of both new ones.
func TestASlowAnswerStillReportsAsATimeout(t *testing.T) {
	t.Parallel()
	slow := fmt.Errorf("read Agent Proxy response: %w", context.DeadlineExceeded)
	if got := turnFailureNotice(stageModel, slow); got != noticeTimedOut {
		t.Fatalf("a drained-too-slowly answer reports as %q, want the timeout", got)
	}
	if got := failureCause(slow); got != causeTimeout {
		t.Errorf("failure cause = %q, want %q", got, causeTimeout)
	}
}

// Every notice a member reads about an answered round has to stop claiming the
// backend was unavailable, so the phrase itself is the assertion.
func TestNoAnsweredRoundNoticeClaimsUnavailability(t *testing.T) {
	t.Parallel()
	answered := map[string]string{
		"too large":  noticeResponseTooLarge,
		"unreadable": noticeResponseUnreadable,
		"rejected":   noticeModelRejected,
		"rounds":     noticeRoundsSpent,
		"budget":     noticeBudgetSpent,
		"refused":    noticeReplyBlocked,
	}
	for name, notice := range answered {
		if strings.Contains(notice, "unavailable") {
			t.Errorf("the %s notice %q claims the backend was unavailable", name, notice)
		}
		if !noticeShape.MatchString(notice) {
			t.Errorf("the %s notice %q is not a harness notice", name, notice)
		}
	}
}

// The two new causes are distinct label values, or a dashboard cannot separate
// an oversized answer from an outage, which is what made this invisible.
func TestTheAnsweredCausesAreTheirOwnLabels(t *testing.T) {
	t.Parallel()
	seen := map[string]bool{}
	for _, cause := range []string{
		causeTimeout, causeModelSilent, causeModelRejected, causeToolFailed,
		causeRoundsSpent, causeReplyRefused, causeBudgetSpent,
		causeResponseTooLarge, causeResponseUnreadable, causeStage, causeShutdown,
	} {
		if seen[cause] {
			t.Fatalf("cause %q is declared twice, so two conditions collapse", cause)
		}
		seen[cause] = true
	}
}
