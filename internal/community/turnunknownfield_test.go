package community

import (
	"fmt"
	"net/http"
	"strings"
	"testing"
)

// A caller who sets a field and gets a 200 concludes the field took effect.
// Issue 173: unknown fields are dropped in silence. These rows pin that.

// unknownFieldRow is one body carrying a field the endpoint does not define.
type unknownFieldRow struct {
	name string
	body string
	// rejectedNow is the behavior on origin/main, which the test asserts so CI
	// reports what ships. shouldReject is what issue 173 asks for.
	rejectedNow  bool
	shouldReject bool
	issue        string
}

var unknownFieldRows = []unknownFieldRow{
	{
		name:        "user_id from the report",
		body:        `{"author":"m","content":"hi","user_id":"1024000000000000001"}`,
		rejectedNow: false, shouldReject: true, issue: "173",
	},
	{
		name:        "session_id from the report",
		body:        `{"author":"m","content":"hi","session_id":"abc"}`,
		rejectedNow: false, shouldReject: true, issue: "173",
	},
	{
		// The sharper case. A typo of an optional field cannot be caught by the
		// content-required check, so nothing else in the handler can see it.
		name:        "typo of an optional field",
		body:        `{"author":"m","content":"hi","request_i":"r-1"}`,
		rejectedNow: false, shouldReject: true, issue: "173",
	},
	{
		name:        "a field that looks authoritative",
		body:        `{"author":"m","content":"hi","principal":"kai"}`,
		rejectedNow: false, shouldReject: true, issue: "173",
	},
}

// Characterization. Each row is accepted today and issue 173 asks for a 400.
func TestTurnSilentlyAcceptsUnknownFields(t *testing.T) {
	t.Parallel()
	agent := httpTurnAgent(t)
	for index, row := range unknownFieldRows {
		recorder := postTurn(t, agent, fmt.Sprintf("unknown-%d", index), row.body)
		rejected := recorder.Code != http.StatusOK
		if rejected == row.rejectedNow {
			continue
		}
		if !row.rejectedNow {
			t.Errorf("%s is now rejected with %d. If issue %s was delivered, set "+
				"rejectedNow to true and clear the issue field", row.name, recorder.Code, row.issue)
			continue
		}
		t.Errorf("regression: %s is accepted again", row.name)
	}
}

// The must-not-fire half. Every field the endpoint defines has to keep working,
// or the fix for 173 breaks the callers it is meant to protect.
func TestTurnAcceptsEveryFieldItDefines(t *testing.T) {
	t.Parallel()
	agent := httpTurnAgent(t)
	for index, body := range []string{
		`{"author":"m","content":"hi"}`,
		`{"author":"m","content":"hi","request_id":"r-1"}`,
		`{"author":"m","content":"hi","history":[]}`,
		`{"author":"m","content":"hi","history":[{"author":"m","content":"earlier"}]}`,
		`{"request_id":"r-2","author":"m","content":"hi","history":[]}`,
	} {
		recorder := postTurn(t, agent, fmt.Sprintf("defined-%d", index), body)
		if recorder.Code != http.StatusOK {
			t.Errorf("a well-formed caller was rejected with %d: %s\n  body %s",
				recorder.Code, strings.TrimSpace(recorder.Body.String()), body)
		}
	}
}

// Acceptance criterion three. A caller cannot currently tell malformed JSON
// from an oversized body, so a third case would collapse into the same reply.
func TestTurnRejectionsAreNotDistinguishableToACaller(t *testing.T) {
	t.Parallel()
	agent := httpTurnAgent(t)
	malformed := postTurn(t, agent, "shape-1", `{"author":`)
	oversized := postTurn(t, agent, "shape-2",
		`{"author":"m","content":"`+strings.Repeat("x", maxHTTPBody+1)+`"}`)

	for _, recorder := range []int{malformed.Code, oversized.Code} {
		if recorder != http.StatusBadRequest {
			t.Fatalf("status = %d, want 400 for both shapes", recorder)
		}
	}
	// Characterization. The exception code reaches the telemetry span only, so
	// both bodies read identically and issue 173's third criterion needs more.
	if malformed.Body.String() != oversized.Body.String() {
		t.Errorf("malformed and oversized are now distinguishable: %q vs %q. If "+
			"issue 157 was delivered, this test should assert they differ",
			strings.TrimSpace(malformed.Body.String()),
			strings.TrimSpace(oversized.Body.String()))
	}
}
