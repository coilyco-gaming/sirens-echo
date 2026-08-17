package community

import (
	"context"
	"net/http"
	"testing"
)

// A backend that answered and refused the request is not a backend that was
// down, and telling a member to retry one of them is wrong. See sirens-echo#875.

func TestAMalformedRequestIsNotReportedAsAnOutage(t *testing.T) {
	t.Parallel()
	rejected := modelHTTPError{Status: http.StatusBadRequest}

	notice := turnFailureNotice(stageModel, rejected)
	if notice == noticeModelFailed {
		t.Fatalf("a 400 still reports as %q, which invites a retry that "+
			"rebuilds the same request", notice)
	}
	if notice != noticeModelRejected {
		t.Errorf("notice = %q", notice)
	}
	if got := failureCause(rejected); got != causeModelRejected {
		t.Errorf("failure cause = %q, want %q", got, causeModelRejected)
	}
	if !noticeShape.MatchString(notice) {
		t.Errorf("%q is not a harness notice", notice)
	}
}

// An outage must keep saying so, or this trade one misdirection for another.
func TestAnUnavailableBackendStillReportsAsOne(t *testing.T) {
	t.Parallel()
	for _, status := range []int{
		http.StatusBadGateway,
		http.StatusServiceUnavailable,
		http.StatusGatewayTimeout,
	} {
		failure := modelHTTPError{Status: status}
		if got := turnFailureNotice(stageModel, failure); got != noticeModelFailed {
			t.Errorf("%d reports as %q, want the unavailable notice", status, got)
		}
		if !retryableModel(failure) {
			t.Errorf("%d stopped being retryable", status)
		}
	}
}

// The two the harness can wait out stay retryable, so a rate limit does not
// become a permanent refusal.
func TestRateLimitAndRequestTimeoutAreNotRejections(t *testing.T) {
	t.Parallel()
	for _, status := range []int{http.StatusTooManyRequests, http.StatusRequestTimeout} {
		if rejectedByModel(modelHTTPError{Status: status}) {
			t.Errorf("%d classified as a malformed request", status)
		}
	}
	if !retryableModel(modelHTTPError{Status: http.StatusTooManyRequests}) {
		t.Error("a rate limit stopped being retryable")
	}
}

// A rejection is never retried, because the retry rebuilds the same request.
func TestARejectedRequestIsNeverRetried(t *testing.T) {
	t.Parallel()
	for _, status := range []int{400, 404, 422} {
		failure := modelHTTPError{Status: status}
		if !rejectedByModel(failure) {
			t.Errorf("%d was not classified as a rejection", status)
		}
		if retryableModel(failure) {
			t.Errorf("%d is retryable, so the same malformed request goes twice", status)
		}
	}
}

// Nothing that is not a model HTTP status becomes a rejection, or a transport
// failure would stop being reported as one.
func TestOnlyAModelStatusBecomesARejection(t *testing.T) {
	t.Parallel()
	if rejectedByModel(context.DeadlineExceeded) {
		t.Error("a deadline classified as a malformed request")
	}
	if rejectedByModel(ErrModelSilent) {
		t.Error("silence classified as a malformed request")
	}
	if rejectedByModel(nil) {
		t.Error("no error classified as a malformed request")
	}
}
