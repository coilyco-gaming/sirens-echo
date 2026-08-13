package community

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"testing"
	"time"
)

// Availability is retried. A malformed request is not, because retrying it
// fails identically four times. See sirens-echo#650.

func TestAvailabilityFailuresAreRetried(t *testing.T) {
	t.Parallel()
	for name, status := range map[string]int{
		"rate limited":    http.StatusTooManyRequests,
		"bad gateway":     http.StatusBadGateway,
		"unavailable":     http.StatusServiceUnavailable,
		"gateway timeout": http.StatusGatewayTimeout,
	} {
		if !retryableModel(modelHTTPError{Status: status}) {
			t.Errorf("%s (%d) was not retried", name, status)
		}
	}
}

// The 400 that motivated this issue's neighbour is a prompt-trimming defect.
// Retrying it produces the same 400 four times and delays the notice.
func TestARequestFaultIsNotRetried(t *testing.T) {
	t.Parallel()
	for name, status := range map[string]int{
		"malformed":     http.StatusBadRequest,
		"unauthorized":  http.StatusUnauthorized,
		"forbidden":     http.StatusForbidden,
		"not found":     http.StatusNotFound,
		"unprocessable": http.StatusUnprocessableEntity,
	} {
		if retryableModel(modelHTTPError{Status: status}) {
			t.Errorf("%s (%d) was retried, which fails identically", name, status)
		}
	}
}

// A transport error never reached a server, so it carries no status and is the
// availability case this exists for.
func TestATransportErrorIsRetried(t *testing.T) {
	t.Parallel()
	if !retryableModel(errors.New("dial tcp: connection refused")) {
		t.Error("a connection failure was not retried")
	}
	if !retryableModel(fmt.Errorf("wrapped: %w", errors.New("EOF"))) {
		t.Error("a wrapped transport error was not retried")
	}
}

// The turn giving up is not an availability problem, and retrying past it
// would spend a budget the turn no longer has.
func TestACancelledTurnIsNotRetried(t *testing.T) {
	t.Parallel()
	for name, err := range map[string]error{
		"cancelled": context.Canceled,
		"expired":   context.DeadlineExceeded,
		"wrapped":   fmt.Errorf("model call: %w", context.DeadlineExceeded),
	} {
		if retryableModel(err) {
			t.Errorf("%s was retried, spending a budget the turn has already lost", name)
		}
	}
}

func TestNoErrorIsNotRetried(t *testing.T) {
	t.Parallel()
	if retryableModel(nil) {
		t.Error("a success was treated as retryable")
	}
}

// The whole ladder has to fit inside the turn ceiling, or a retry converts a
// fast failure into a timeout. See sirens-echo#577.
func TestTheLadderFitsInsideTheTurnCeiling(t *testing.T) {
	t.Parallel()
	total := time.Duration(0)
	backoff := modelRetryBackoff
	for attempt := 1; attempt < modelRetryAttempts; attempt++ {
		total += backoff
		backoff *= 2
	}
	if total >= defaultRequestTimeout {
		t.Fatalf("backoff totals %s against a %s turn ceiling", total, defaultRequestTimeout)
	}
	// Well inside, not merely inside: the attempts themselves need the rest.
	if total > defaultRequestTimeout/10 {
		t.Errorf("backoff totals %s, more than a tenth of the %s ceiling",
			total, defaultRequestTimeout)
	}
}
