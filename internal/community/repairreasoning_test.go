package community

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"
	"time"
)

// The tool-call path is guarded. The repair path builds its own assistant
// message, was not covered, and that is how sirens-echo#678 survived.

// reasoningRepairServer replays a style violation carrying reasoning content,
// then hands the repair round's assistant message back to the caller.
func reasoningRepairServer(t *testing.T, reasoning string, seen *string) *httptest.Server {
	t.Helper()
	var round atomic.Int32
	return httptest.NewServer(http.HandlerFunc(func(
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
			_, _ = writer.Write([]byte(
				`{"choices":[{"message":{"content":"Hey there! Happy to help.",` +
					`"reasoning_content":"` + reasoning + `"}}]}`,
			))
		case 2:
			if len(body.Messages) != 4 || body.Messages[2].Role != "assistant" {
				t.Errorf("repair messages = %#v", body.Messages)
			} else {
				*seen = body.Messages[2].ReasoningContent
			}
			_, _ = writer.Write([]byte(
				`{"choices":[{"message":{"content":"repaired"}}]}`,
			))
		default:
			http.Error(writer, "unexpected model round", http.StatusInternalServerError)
		}
	}))
}

// The repair path carries reasoning content, so a thinking-mode model is not
// sent an assistant message it refuses. See sirens-echo#678.
func TestTheRepairPathKeepsReasoningContent(t *testing.T) {
	t.Parallel()
	const reasoning = "The member greeted me, so a short greeting is enough."
	seen := "not set"
	server := reasoningRepairServer(t, reasoning, &seen)
	defer server.Close()

	client := ProxyClient{
		BaseURL:     server.URL,
		Model:       "deepseek-reasoner",
		AuditRole:   "community",
		Attribution: "Sirens Echo",
		HTTPClient:  &http.Client{Timeout: time.Second},
	}
	got, err := client.Complete(
		context.Background(),
		TurnPrompt{System: "system", Message: "user"},
		"request",
	)
	if err != nil {
		t.Fatalf("Complete: %v", err)
	}
	if got.Content != "repaired" {
		t.Fatalf("completion = %q", got.Content)
	}

	if seen == "not set" {
		t.Fatal("the repair round never reached the assertion")
	}
	// Empty and absent are the same bytes under omitempty, so an assertion that
	// the key is present cannot distinguish them. Compare the value.
	if seen != reasoning {
		t.Errorf("the repair round sent reasoning content %q, want %q", seen, reasoning)
	}
}
