package community

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"
	"time"
)

// The repair path now carries reasoning content. A model that returns none
// still produces an assistant message with no such key. See sirens-echo#678.

// assistantKeys returns the JSON keys of the first assistant message in a raw
// request body. Decoding first would erase the absent-against-empty difference.
func assistantKeys(t *testing.T, raw []byte) map[string]bool {
	t.Helper()
	var body struct {
		Messages []map[string]json.RawMessage `json:"messages"`
	}
	if err := json.Unmarshal(raw, &body); err != nil {
		t.Fatalf("decode outgoing body: %v", err)
	}
	for _, message := range body.Messages {
		var role string
		if err := json.Unmarshal(message["role"], &role); err != nil || role != "assistant" {
			continue
		}
		keys := make(map[string]bool, len(message))
		for key := range message {
			keys[key] = true
		}
		return keys
	}
	return nil
}

// A tool round whose model turn carried no reasoning. DeepSeek in thinking mode
// rejects the array this produces, which is what sirens-echo#678 reported.
func TestAnEmptyReasoningContentLeavesNoKey(t *testing.T) {
	t.Parallel()
	var round atomic.Int32
	var keys map[string]bool
	server := httptest.NewServer(http.HandlerFunc(func(
		writer http.ResponseWriter,
		request *http.Request,
	) {
		raw, err := io.ReadAll(request.Body)
		if err != nil {
			t.Errorf("read body: %v", err)
		}
		writer.Header().Set("Content-Type", "application/json")
		switch round.Add(1) {
		case 1:
			_, _ = writer.Write([]byte(
				`{"choices":[{"message":{"content":null,"reasoning_content":"",` +
					`"tool_calls":[{"id":"call-1","type":"function","function":` +
					`{"name":"eco__get_eco_server_status","arguments":"{}"}}]}}]}`,
			))
		case 2:
			keys = assistantKeys(t, raw)
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
		Model:       "deepseek-reasoner",
		AuditRole:   "community",
		Attribution: "Sirens Echo",
		HTTPClient:  &http.Client{Timeout: time.Second},
		Tools:       fixtureToolProvider{},
	}
	if _, err := client.Complete(
		context.Background(),
		TurnPrompt{System: "system", Message: "user"},
		"request",
	); err != nil {
		t.Fatalf("Complete: %v", err)
	}
	if keys == nil {
		t.Fatal("no assistant message reached the second round")
	}

	// Asserted as it ships. The fix is not obviously dropping omitempty, since
	// nobody has established whether the provider accepts an empty string.
	if keys["reasoning_content"] {
		t.Errorf("the assistant message now carries reasoning_content when the "+
			"model returned none, keys = %v. If sirens-echo#678 was closed, "+
			"invert this assertion and record what the provider accepts", keys)
	}
	if !keys["tool_calls"] || !keys["role"] {
		t.Errorf("the assistant message lost a key this test relies on: %v", keys)
	}
}
