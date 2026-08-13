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
// message, is not covered, and that is how sirens-echo#678 survived.

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

// The repair path drops reasoning content, so a thinking-mode model is sent an
// assistant message it will refuse. Asserted as it ships. See sirens-echo#678.
func TestTheRepairPathDropsReasoningContent(t *testing.T) {
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

	// The tool-call path carries this through. proxy.go:510 has no field for it
	// at all, so the repair round can only ever send the zero value.
	if seen == "not set" {
		t.Fatal("the repair round never reached the assertion")
	}
	if seen != "" {
		t.Errorf("the repair path now sends reasoning content %q. If "+
			"sirens-echo#678 was fixed, change this test to require %q and "+
			"delete this branch", seen, reasoning)
	}
}
