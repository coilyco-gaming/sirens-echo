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

// An assistant message echoes the reasoning key exactly when the response
// carried it. See docs/sirens-echo-reasoning-roundtrip.md and sirens-echo#717.

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

// A tool round whose model turn carried an empty reasoning string. DeepSeek in
// thinking mode rejected the array this used to produce. See sirens-echo#678.
func TestAnEmptyReasoningContentStillSendsTheKey(t *testing.T) {
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

	// The response named the field as an empty string, so the echo carries it.
	// Absent, the provider rejects the array as malformed. See sirens-echo#678.
	if !keys["reasoning_content"] {
		t.Errorf("the assistant message dropped reasoning_content the model "+
			"returned as an empty string, keys = %v", keys)
	}
	if !keys["tool_calls"] || !keys["role"] {
		t.Errorf("the assistant message lost a key this test relies on: %v", keys)
	}
}

// The other half of the round trip, and the reason this is not a bare drop of
// omitempty: a provider that never names the field is not echoed one.
func TestAnUnnamedReasoningFieldIsNotInvented(t *testing.T) {
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
			// No reasoning_content key at all, which is every non-thinking model
			// on every other lane. Their requests must not change shape.
			_, _ = writer.Write([]byte(
				`{"choices":[{"message":{"content":null,` +
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
		Model:       "a-model-that-does-not-think",
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
	if keys["reasoning_content"] {
		t.Errorf("a model that never named reasoning_content was echoed one, "+
			"keys = %v. That changes every request on the healthy lanes", keys)
	}
}

// Only assistant messages ever carry it. Dropping omitempty from the shared
// struct would have stamped the key onto system, user, and tool messages too.
func TestOnlyAssistantMessagesCarryReasoning(t *testing.T) {
	t.Parallel()
	reasoning := ""
	encoded, err := json.Marshal(chatRequest{Messages: []chatMessage{
		{Role: "system", Content: "system"},
		{Role: "user", Content: "user"},
		{Role: "assistant", Content: "reply", ReasoningContent: &reasoning},
		{Role: "tool", Content: "result", ToolCallID: "call-1", Name: "a_tool"},
	}})
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	var body struct {
		Messages []map[string]json.RawMessage `json:"messages"`
	}
	if err := json.Unmarshal(encoded, &body); err != nil {
		t.Fatalf("decode: %v", err)
	}
	for index, message := range body.Messages {
		var role string
		if err := json.Unmarshal(message["role"], &role); err != nil {
			t.Fatalf("decode role %d: %v", index, err)
		}
		_, carried := message["reasoning_content"]
		if want := role == "assistant"; carried != want {
			t.Errorf("message %d role %q carries reasoning_content = %v, want %v",
				index, role, carried, want)
		}
	}
}
