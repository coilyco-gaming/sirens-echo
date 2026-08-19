package community

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
	"time"
)

// A turn with no allowance installed is every caller outside a turn: the
// evaluation board, the bridge, and the rate lane must keep their tools.
func TestNilTurnBudgetAffordsEverything(t *testing.T) {
	t.Parallel()
	var absent *turnBudget
	if !absent.affordsToolRound() {
		t.Error("a nil budget withdrew tools, so a caller outside a turn lost them")
	}
	if got := turnBudgetFrom(context.Background()); got != nil {
		t.Errorf("a bare context carried %v, want nil", got)
	}
	if got := turnBudgetFrom(withTurnBudget(context.Background(), 0)); got != nil {
		t.Error("a non-positive allowance installed a budget, so a lane that " +
			"has not opted in changed behaviour")
	}
}

// A tool round costs two calls, so one left buys an answer and not a tool.
func TestTurnBudgetStopsAffordingToolsWithOneCallLeft(t *testing.T) {
	t.Parallel()
	budget := turnBudgetFrom(withTurnBudget(context.Background(), 3))
	if budget == nil {
		t.Fatal("withTurnBudget installed nothing")
	}
	if !budget.affordsToolRound() {
		t.Error("three calls did not afford a tool round")
	}
	budget.spend()
	if !budget.affordsToolRound() {
		t.Error("two calls did not afford a tool round")
	}
	budget.spend()
	if budget.affordsToolRound() {
		t.Error("one call afforded a tool round, so the answer has nothing to spend on")
	}
	if left := budget.spend(); left != 0 {
		t.Errorf("spending the last call left %d, want 0", left)
	}
}

// The bug this exists for: a turn makes several completions and each one
// carried its own ceiling, so the per-call bound multiplied. See #1076.
func TestTurnBudgetIsSharedAcrossCompletionsInOneTurn(t *testing.T) {
	t.Parallel()
	var rounds atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(
		writer http.ResponseWriter,
		request *http.Request,
	) {
		var body chatRequest
		if err := json.NewDecoder(request.Body).Decode(&body); err != nil {
			t.Errorf("decode body: %v", err)
		}
		rounds.Add(1)
		writer.Header().Set("Content-Type", "application/json")
		// Tools withdrawn is the only way out of this handler, so a completion
		// that keeps them keeps looping and the count is the bound under test.
		if len(body.Tools) == 0 {
			_, _ = writer.Write([]byte(
				`{"choices":[{"message":{"content":"Answered from what was gathered."}}]}`,
			))
			return
		}
		_, _ = writer.Write([]byte(
			`{"choices":[{"message":{"tool_calls":[{"id":"c1","type":"function",` +
				`"function":{"name":"probe__look","arguments":"{}"}}]}}]}`,
		))
	}))
	defer server.Close()

	client := ProxyClient{
		BaseURL:     server.URL,
		Model:       "selected-model",
		AuditRole:   "community",
		Attribution: "Sirens Echo",
		Tools:       alwaysOneTool{},
		HTTPClient:  &http.Client{Timeout: 5 * time.Second},
	}
	// Below one completion's own ceiling, so the allowance is what binds and a
	// pass cannot come from maxToolRounds happening to be smaller.
	const allowance = 4
	ctx := withTurnBudget(context.Background(), allowance)
	for completion := 1; completion <= 3; completion++ {
		if _, err := client.Complete(ctx, TurnPrompt{System: "s", Message: "u"}, "request"); err != nil {
			t.Fatalf("completion %d returned an error instead of a degraded answer: %v",
				completion, err)
		}
	}
	// The allowance funds the tool rounds. Every completion still buys its own
	// answer, which is deliberate: a starved filing check is the worse failure.
	if got := int(rounds.Load()); got > allowance+3 {
		t.Errorf("three completions spent %d model calls against an allowance of %d, "+
			"so each one restarted the ceiling", got, allowance)
	}
	if got := int(rounds.Load()); got <= 3 {
		t.Errorf("three completions spent only %d model calls, so the allowance "+
			"withdrew tools before the turn had used any", got)
	}
}

// Withdrawing tools has to say why, or the model answers from nothing and
// reports no gap. The notice is the same one the per-call ceiling sends.
func TestTurnBudgetExhaustionCarriesTheSpentNotice(t *testing.T) {
	t.Parallel()
	var sawNotice atomic.Bool
	server := httptest.NewServer(http.HandlerFunc(func(
		writer http.ResponseWriter,
		request *http.Request,
	) {
		var body chatRequest
		if err := json.NewDecoder(request.Body).Decode(&body); err != nil {
			t.Errorf("decode body: %v", err)
		}
		writer.Header().Set("Content-Type", "application/json")
		if len(body.Tools) == 0 {
			for _, message := range body.Messages {
				if text, ok := message.Content.(string); ok &&
					strings.Contains(text, "tool budget for this turn is spent") {
					sawNotice.Store(true)
				}
			}
			_, _ = writer.Write([]byte(
				`{"choices":[{"message":{"content":"Answered from what was gathered."}}]}`,
			))
			return
		}
		_, _ = writer.Write([]byte(
			`{"choices":[{"message":{"tool_calls":[{"id":"c1","type":"function",` +
				`"function":{"name":"probe__look","arguments":"{}"}}]}}]}`,
		))
	}))
	defer server.Close()

	client := ProxyClient{
		BaseURL:     server.URL,
		Model:       "selected-model",
		AuditRole:   "community",
		Attribution: "Sirens Echo",
		Tools:       alwaysOneTool{},
		HTTPClient:  &http.Client{Timeout: 5 * time.Second},
	}
	got, err := client.Complete(
		withTurnBudget(context.Background(), 3),
		TurnPrompt{System: "s", Message: "u"},
		"request",
	)
	if err != nil {
		t.Fatalf("Complete returned an error instead of a degraded answer: %v", err)
	}
	if got.Content != "Answered from what was gathered." {
		t.Fatalf("content = %q", got.Content)
	}
	if !sawNotice.Load() {
		t.Error("tools were withdrawn without the spent-budget notice, so the " +
			"model was asked to answer with no account of the gap")
	}
	if len(got.ToolCalls) == 0 {
		t.Error("the allowance withdrew tools before any round ran, so the " +
			"degraded answer has nothing gathered behind it")
	}
}

// A lane may tune the allowance down and must not be able to tune it into a
// toolless lane by accident.
func TestModelBudgetRejectsATurnAllowanceThatFundsNoToolRound(t *testing.T) {
	t.Parallel()
	if err := (ModelBudget{TurnModelCalls: 1}).validate(); err == nil {
		t.Error("turn_model_calls of 1 validated, so a lane can withdraw tools " +
			"from every turn without saying so")
	}
	if err := (ModelBudget{TurnModelCalls: 2}).validate(); err != nil {
		t.Errorf("turn_model_calls of 2 funds one tool round and was refused: %v", err)
	}
	if err := (ModelBudget{}).validate(); err != nil {
		t.Errorf("an unset budget no longer validates: %v", err)
	}
	if got := (ModelBudget{}).resolved().TurnModelCalls; got != turnModelCalls {
		t.Errorf("an unset turn allowance resolved to %d, want the packaged %d",
			got, turnModelCalls)
	}
}
