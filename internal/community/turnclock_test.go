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

// Echo had no clock, so no question about the current time was answerable and
// no epoch was available to render from. See sirens-echo#855.
func TestTheTurnCarriesTheCurrentTime(t *testing.T) {
	t.Parallel()
	moment := time.Date(2026, 8, 16, 2, 55, 21, 0, time.UTC)
	var captured atomic.Value
	server := httptest.NewServer(http.HandlerFunc(func(
		writer http.ResponseWriter, request *http.Request,
	) {
		var body chatRequest
		if err := json.NewDecoder(request.Body).Decode(&body); err != nil {
			t.Errorf("decode body: %v", err)
		}
		captured.Store(body.Messages)
		writer.Header().Set("Content-Type", "application/json")
		_, _ = writer.Write([]byte(`{"choices":[{"message":{"content":"Answered."}}]}`))
	}))
	defer server.Close()

	client := ProxyClient{
		BaseURL:    server.URL,
		Model:      "model",
		AuditRole:  "community",
		HTTPClient: &http.Client{Timeout: 2 * time.Second},
		Now:        func() time.Time { return moment },
	}
	if _, err := client.Complete(
		context.Background(),
		TurnPrompt{System: "system", Message: "what time is it"},
		"request",
	); err != nil {
		t.Fatalf("Complete: %v", err)
	}

	messages, _ := captured.Load().([]chatMessage)
	if len(messages) < 2 {
		t.Fatalf("messages = %#v", messages)
	}
	// Directly under the local policy, so it reads as a fact about the turn
	// rather than as reference material a server published.
	clock := messages[1]
	if clock.Role != "system" {
		t.Errorf("clock role = %q", clock.Role)
	}
	content, _ := clock.Content.(string)
	if !strings.Contains(content, "2026-08-16 02:55:21 UTC") {
		t.Errorf("the turn carries no readable time: %q", content)
	}
	// The epoch is the half the discord-timestamps resource needs and could
	// never supply, since rendering markup and knowing the moment are separate.
	if !strings.Contains(content, "1786848921") {
		t.Errorf("the turn carries no epoch to render from: %q", content)
	}
}

// One clock per turn, not one per round, or two tool rounds would disagree
// about what time it is inside one answer.
func TestTheClockDoesNotAdvanceBetweenRounds(t *testing.T) {
	t.Parallel()
	var reads atomic.Int32
	var round atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(
		writer http.ResponseWriter, request *http.Request,
	) {
		var body chatRequest
		_ = json.NewDecoder(request.Body).Decode(&body)
		writer.Header().Set("Content-Type", "application/json")
		if round.Add(1) == 1 {
			_, _ = writer.Write([]byte(`{"choices":[{"message":{"content":"Hey there! Happy to help."}}]}`))
			return
		}
		_, _ = writer.Write([]byte(`{"choices":[{"message":{"content":"Answered."}}]}`))
	}))
	defer server.Close()

	client := ProxyClient{
		BaseURL:    server.URL,
		Model:      "model",
		AuditRole:  "community",
		HTTPClient: &http.Client{Timeout: 2 * time.Second},
		Now: func() time.Time {
			reads.Add(1)
			return time.Date(2026, 8, 16, 2, 55, 21, 0, time.UTC)
		},
	}
	if _, err := client.Complete(
		context.Background(), TurnPrompt{System: "system", Message: "hello"}, "request",
	); err != nil {
		t.Fatalf("Complete: %v", err)
	}
	if round.Load() < 2 {
		t.Fatalf("the repair round did not run, so this test asserts nothing")
	}
	if got := reads.Load(); got != 1 {
		t.Errorf("the clock was read %d times in one turn", got)
	}
}
