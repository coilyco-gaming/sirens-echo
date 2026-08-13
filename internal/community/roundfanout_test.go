package community

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
	"time"
)

// tool_rounds bounds rounds and tool_result_bytes bounds one call. Nothing
// bounds calls within a round, so a turn's total is unbounded. See #635.

// fanOutRound returns one assistant turn requesting the same tool many times,
// which is the shape a wide fan-out takes on the wire.
func fanOutRound(calls int) string {
	requested := make([]string, 0, calls)
	for index := range calls {
		requested = append(requested, fmt.Sprintf(
			`{"id":"call-%d","type":"function","function":`+
				`{"name":"eco__get_eco_server_status","arguments":"{}"}}`, index))
	}
	return `{"choices":[{"message":{"content":null,"tool_calls":[` +
		strings.Join(requested, ",") + `]}}]}`
}

// A round of twenty calls runs all twenty. The per-call bound applies to each,
// so the turn reinjects twenty times it. See sirens-echo#635.
func TestARoundRunsEveryCallItIsGiven(t *testing.T) {
	t.Parallel()
	const requested = 20
	var round atomic.Int32
	var results int
	server := httptest.NewServer(http.HandlerFunc(func(
		writer http.ResponseWriter,
		request *http.Request,
	) {
		var body chatRequest
		if err := json.NewDecoder(request.Body).Decode(&body); err != nil {
			t.Errorf("decode body: %v", err)
		}
		writer.Header().Set("Content-Type", "application/json")
		switch round.Add(1) {
		case 1:
			_, _ = writer.Write([]byte(fanOutRound(requested)))
		case 2:
			for _, message := range body.Messages {
				if message.Role == "tool" {
					results++
				}
			}
			_, _ = writer.Write([]byte(
				`{"choices":[{"message":{"content":"Eco is online now."}}]}`,
			))
		default:
			http.Error(writer, "unexpected model round", http.StatusInternalServerError)
		}
	}))
	defer server.Close()

	client := ProxyClient{
		BaseURL:     server.URL,
		Model:       "selected-model",
		AuditRole:   "community",
		Attribution: "Sirens Echo",
		HTTPClient:  &http.Client{Timeout: 5 * time.Second},
		Tools:       fixtureToolProvider{},
	}
	if _, err := client.Complete(
		context.Background(),
		TurnPrompt{System: "system", Message: "user"},
		"request",
	); err != nil {
		t.Fatalf("Complete: %v", err)
	}

	// Asserted as it ships. A cap would make this fewer, which is the change
	// sirens-echo#635's first criterion would bring.
	if results != requested {
		t.Errorf("a round of %d calls delivered %d tool results. If a "+
			"per-round cap landed, sirens-echo#635 is being acted on and this "+
			"test should assert the cap instead", requested, results)
	}
}
