package community

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

// A model that deliberates past its ceiling and emits nothing is not a backend
// outage. See docs/sirens-echo-model-call.md and sirens-echo#549.

// The failure carried no sentinel, so every consumer saw an opaque string and
// the condition collapsed into stage_failed.
func TestBudgetExhaustionIsIdentifiableByItsCause(t *testing.T) {
	t.Parallel()
	err := formatBudgetExhausted(3600, 1, 16609)
	if !errors.Is(err, ErrBudgetExhausted) {
		t.Fatal("the budget failure does not identify itself")
	}
	// The rendered numbers are the diagnosis and must survive the wrapping.
	rendered := err.Error()
	for _, want := range []string{"3600 tokens", "after 1 raises", "16609 bytes"} {
		if !strings.Contains(rendered, want) {
			t.Errorf("the failure lost %q: %q", want, rendered)
		}
	}
}

// The live shape on sirens-echo#549: every model call returned 200, the model
// spent 3600 tokens reasoning, and the member was told the backend was down.
func TestASpentCompletionBudgetDoesNotReadAsAnOutage(t *testing.T) {
	t.Parallel()
	cause := formatBudgetExhausted(3600, 1, 16609)
	got := turnFailureNotice(stageModel, cause)
	if got == noticeModelFailed {
		t.Fatal("a spent completion budget still claims the backend is unavailable")
	}
	if got != noticeBudgetSpent {
		t.Fatalf("notice = %q, want the budget-spent notice", got)
	}
	// A genuine model-stage failure must keep its own notice, or the fix trades
	// one wrong notice for another.
	if turnFailureNotice(stageModel, errors.New("dial tcp: connection refused")) != noticeModelFailed {
		t.Fatal("an ordinary model failure stopped reading as one")
	}
	// Boundary responses stay short. See issue 175.
	if words := countWords(noticeBudgetSpent); words > 12 {
		t.Fatalf("the notice runs to %d words, which is a handle to pull", words)
	}
}

// Two budgets bind a turn and they are different numbers. Sharing one cause
// would make a ceiling decision unanswerable from the failure series.
func TestSpentRoundsAndSpentTokensAreSeparateCauses(t *testing.T) {
	t.Parallel()
	rounds := failureCause(fmt.Errorf("spent: %w", ErrToolRoundsExhausted))
	tokens := failureCause(formatBudgetExhausted(3600, 1, 16609))
	if rounds == tokens {
		t.Fatalf("both classified as %q", rounds)
	}
	if tokens != causeBudgetSpent {
		t.Errorf("cause = %q, want %q", tokens, causeBudgetSpent)
	}
	// It must also leave the generic bucket, which is the defect being fixed.
	if tokens == causeStage {
		t.Error("the condition is still collapsed into the stage")
	}
}

// The label and the phrase a member reads are picked by the same order, so a
// notice can never describe a condition the telemetry classified differently.
func TestTheBudgetCauseAgreesWithTheBudgetNotice(t *testing.T) {
	t.Parallel()
	cause := formatBudgetExhausted(14400, 2, 40000)
	if got := failureCause(cause); got != causeBudgetSpent {
		t.Errorf("cause = %q, want %q", got, causeBudgetSpent)
	}
	if got := turnFailureNotice(stageModel, cause); got != noticeBudgetSpent {
		t.Errorf("notice = %q, want the budget-spent notice", got)
	}
}

// The whole path, because the sentinel is only worth anything if it survives
// the climb and reaches the caller that picks the member's notice.
func TestAnAlwaysTruncatingProxyEndsTheTurnAsBudgetSpent(t *testing.T) {
	t.Parallel()
	calls := 0
	server := httptest.NewServer(http.HandlerFunc(
		func(writer http.ResponseWriter, _ *http.Request) {
			calls++
			writer.Header().Set("Content-Type", "application/json")
			// Whole budget on reasoning, nothing to say. The shape sirens-echo#549
			// captured in production and on both batteries.
			_, _ = writer.Write([]byte(
				`{"choices":[{"finish_reason":"length","message":` +
					`{"content":"","reasoning_content":"deliberating at length"}}]}`,
			))
		}))
	defer server.Close()

	client := ProxyClient{
		BaseURL:     server.URL,
		Model:       "reasoning-model",
		AuditRole:   "community",
		Attribution: "Sirens Echo",
		Harness:     transportDiscord,
		HTTPClient:  &http.Client{Timeout: time.Second},
	}
	_, err := client.Complete(
		context.Background(), TurnPrompt{System: "system", Message: "user"}, "request",
	)
	if err == nil {
		t.Fatal("a turn that emitted nothing succeeded")
	}
	if !errors.Is(err, ErrBudgetExhausted) {
		t.Fatalf("the turn failed as %v, which no consumer can classify", err)
	}
	// It climbed before giving up, or the ceiling was never the thing that bound it.
	if want := modelCallsBeforeBudgetSpent((ProxyClient{}).budget()); calls != want {
		t.Errorf("made %d model calls, want %d", calls, want)
	}
	if got := failureCause(err); got != causeBudgetSpent {
		t.Errorf("cause = %q, want %q", got, causeBudgetSpent)
	}
	if got := turnFailureNotice(stageModel, err); got != noticeBudgetSpent {
		t.Errorf("notice = %q, want the budget-spent notice", got)
	}
}

// The reasoning byte count is the diagnosis and the reasoning text is model
// output. Naming the cause must not have turned one into the other.
func TestTheBudgetNoticeCarriesNoReasoningText(t *testing.T) {
	t.Parallel()
	secret := "the operator handle is coilysiren and the plan is"
	cause := formatBudgetExhausted(3600, 1, len(secret))
	notice := turnFailureNotice(stageModel, cause)
	if strings.Contains(notice, "coilysiren") || strings.Contains(notice, "plan") {
		t.Errorf("the notice leaks reasoning text: %q", notice)
	}
	// The notice is a fixed phrase, so it carries no turn numbers either.
	if strings.Contains(notice, "3600") {
		t.Errorf("the notice carries a spend figure: %q", notice)
	}
}
