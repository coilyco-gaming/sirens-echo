package community

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"
)

// The failure sirens-echo#796 names, in the shape of trace
// 7ea1e319b92b0357d3e2ac71b802a66a. Its outage report is the third block.

const groundedBlocks = "- forgejo - answered, 116 open issues on sirens-echo.\n" +
	"- steam - answered, 412 owned titles.\n" +
	"- playwright - down, same notifications/initialized: Bad Request on both " +
	"attempts, so no page loaded."

// inventedBlock is the one block that fails grounding. Nothing supplied this
// channel and no tool reached it.
const inventedBlock = "\n- discord - answered, the roster is posted in #general each cycle."

// scriptedModel answers each call with the next scripted reply, repeating the
// last one, and records every request body so the repair prompt can be read.
type scriptedModel struct {
	mu       sync.Mutex
	requests []chatRequest
	replies  []string
}

func (m *scriptedModel) server(t *testing.T) *httptest.Server {
	t.Helper()
	return httptest.NewServer(http.HandlerFunc(func(
		writer http.ResponseWriter,
		request *http.Request,
	) {
		var body chatRequest
		if err := json.NewDecoder(request.Body).Decode(&body); err != nil {
			t.Errorf("decode body: %v", err)
		}
		m.mu.Lock()
		m.requests = append(m.requests, body)
		reply := m.replies[min(len(m.requests)-1, len(m.replies)-1)]
		m.mu.Unlock()
		encoded, err := json.Marshal(reply)
		if err != nil {
			t.Errorf("marshal reply: %v", err)
			return
		}
		writer.Header().Set("Content-Type", "application/json")
		_, _ = writer.Write([]byte(
			`{"choices":[{"message":{"content":` + string(encoded) + `}}]}`))
	}))
}

func (m *scriptedModel) calls() int {
	m.mu.Lock()
	defer m.mu.Unlock()
	return len(m.requests)
}

// repairPrompt returns the user turn the loop appended after the first refusal.
func (m *scriptedModel) repairPrompt(t *testing.T) string {
	t.Helper()
	m.mu.Lock()
	defer m.mu.Unlock()
	if len(m.requests) < 2 {
		t.Fatalf("the loop made %d model calls, so it never repaired", len(m.requests))
	}
	messages := m.requests[len(m.requests)-1].Messages
	return messages[len(messages)-1].Content.(string)
}

// checkingAgent is the reply checks as the runtime wires them, so a test drives
// the same set the turn does rather than a stand-in.
func checkingAgent() *Agent {
	return &Agent{
		cfg: Config{Definition: Definition{
			Identity:      "Sirens Deep of Coilyco",
			ResponseStyle: ResponseStyleNeutral,
		}},
		telemetry:   telemetryOrNoop(nil),
		identifiers: NewIdentifierGuard(Config{}, nil),
	}
}

// completeWith runs one turn against the scripted replies, with the harness
// checks offered to the repair loop exactly as NewAgent offers them.
func completeWith(t *testing.T, replies ...string) (*scriptedModel, string, error) {
	t.Helper()
	model := &scriptedModel{replies: replies}
	server := model.server(t)
	defer server.Close()

	client := ProxyClient{
		BaseURL:       server.URL,
		Model:         "model",
		AuditRole:     "community",
		Attribution:   "Sirens Deep",
		ResponseStyle: ResponseStyleNeutral,
		HTTPClient:    &http.Client{Timeout: 2 * time.Second},
		ValidateReply: checkingAgent().repairableReplyChecks,
	}
	result, err := client.Complete(
		context.Background(),
		TurnPrompt{System: "system", Message: "query each of your MCPs, 1 simple query each"},
		"request",
	)
	return model, result.Content, err
}

// The acceptance criterion of sirens-echo#796. The eleven correct blocks are
// not destroyed by the twelfth.
func TestACorrectBlockSurvivesAnotherBlocksRefusal(t *testing.T) {
	t.Parallel()
	_, content, err := completeWith(t, groundedBlocks+inventedBlock, groundedBlocks)

	if err != nil {
		t.Fatalf("the repaired reply did not survive: %v", err)
	}
	if !strings.Contains(content, "notifications/initialized: Bad Request") {
		t.Errorf("the outage report was discarded with the invented channel:\n%s", content)
	}
	if strings.Contains(content, "#general") {
		t.Errorf("the invented channel reached the member:\n%s", content)
	}
}

// Rephrasing blind was the cost the issue measured. The model is told which
// clause failed, so it can fix that clause.
func TestTheRepairPromptNamesTheClauseThatFailed(t *testing.T) {
	t.Parallel()
	model, _, err := completeWith(t, groundedBlocks+inventedBlock, groundedBlocks)

	if err != nil {
		t.Fatalf("complete: %v", err)
	}
	if prompt := model.repairPrompt(t); !strings.Contains(prompt, "#general") {
		t.Errorf("the repair prompt does not name the offending token:\n%s", prompt)
	}
}

// The safety invariant. A model that will not fix the clause leaves the turn
// exactly where it stood before this change, under the same rule.
func TestAnUnrepairedReplyIsStillRefusedByTheSameRule(t *testing.T) {
	t.Parallel()
	model, content, err := completeWith(t, groundedBlocks+inventedBlock)

	// Complete does not refuse it. Ending the turn here would report the model
	// failing when it is the harness refusing. See docs/sirens-echo-turn-stages.md.
	if err != nil {
		t.Fatalf("an unrepaired reply ended the turn inside the completion layer: %v", err)
	}
	if model.calls() != 2 {
		t.Errorf("the loop made %d model calls, want one repair attempt and no more",
			model.calls())
	}
	_, refused, checkErr := checkingAgent().runReplyChecks(
		content, TurnPrompt{}, CompletionResult{},
	)
	if checkErr == nil {
		t.Fatal("the reply that still invents a channel was accepted")
	}
	if refused != replyCheckInventedChannel {
		t.Errorf("refused = %q, want %q", refused, replyCheckInventedChannel)
	}
}

// The evaluation harness builds a client with no hook and scores the raw model
// behaviour. A nil hook must not repair, or the rate packs stop measuring it.
func TestAClientWithNoHookDoesNotRepairAHarnessCheck(t *testing.T) {
	t.Parallel()
	model := &scriptedModel{replies: []string{groundedBlocks + inventedBlock}}
	server := model.server(t)
	defer server.Close()

	client := ProxyClient{
		BaseURL:       server.URL,
		Model:         "model",
		AuditRole:     "community",
		Attribution:   "Sirens Deep",
		ResponseStyle: ResponseStyleNeutral,
		HTTPClient:    &http.Client{Timeout: 2 * time.Second},
	}
	result, err := client.Complete(
		context.Background(),
		TurnPrompt{System: "system", Message: "user"},
		"request",
	)
	if err != nil {
		t.Fatalf("complete: %v", err)
	}
	if model.calls() != 1 {
		t.Errorf("the loop made %d model calls, so a nil hook still repaired", model.calls())
	}
	if !strings.Contains(result.Content, "#general") {
		t.Error("the unhooked client altered the reply the scorer measures")
	}
}
