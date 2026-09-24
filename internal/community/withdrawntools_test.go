package community

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"reflect"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

// A repair round sends the same tools as the first, so the rendered prompt
// keeps its cached prefix. sirens-echo#8139.
func TestRepairRoundKeepsTheToolsArrayUnchanged(t *testing.T) {
	t.Parallel()
	var mu sync.Mutex
	var seen [][]chatTool
	var round atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		var body chatRequest
		if err := json.NewDecoder(request.Body).Decode(&body); err != nil {
			t.Errorf("decode body: %v", err)
		}
		mu.Lock()
		seen = append(seen, body.Tools)
		mu.Unlock()
		writer.Header().Set("Content-Type", "application/json")
		if round.Add(1) == 1 {
			_, _ = writer.Write([]byte(`{"choices":[{"message":{"content":"Hey there! Happy to help."}}]}`))
			return
		}
		_, _ = writer.Write([]byte(`{"choices":[{"message":{"content":"Request received."}}]}`))
	}))
	defer server.Close()

	client := ProxyClient{
		BaseURL:       server.URL,
		Model:         "selected-model",
		AuditRole:     "community",
		ResponseStyle: "neutral",
		Tools:         alwaysOneTool{},
		HTTPClient:    &http.Client{Timeout: 5 * time.Second},
	}
	if _, err := client.Complete(context.Background(), TurnPrompt{System: "s", Message: "u"}, "request"); err != nil {
		t.Fatalf("Complete: %v", err)
	}
	if len(seen) != 2 {
		t.Fatalf("model rounds = %d, want a repair round", len(seen))
	}
	if len(seen[1]) == 0 || !reflect.DeepEqual(seen[0], seen[1]) {
		t.Fatalf("repair round tools = %#v, want the first round's %#v", seen[1], seen[0])
	}
}

// A call on a withdrawn round is answered without running, and the model's
// next answer ships.
func TestAWithdrawnRoundRefusesOneToolCallAndStillAnswers(t *testing.T) {
	t.Parallel()
	var round atomic.Int32
	var refusalSeen atomic.Bool
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		var body chatRequest
		if err := json.NewDecoder(request.Body).Decode(&body); err != nil {
			t.Errorf("decode body: %v", err)
		}
		writer.Header().Set("Content-Type", "application/json")
		switch round.Add(1) {
		case 1:
			_, _ = writer.Write([]byte(`{"choices":[{"message":{"content":"Hey there! Happy to help."}}]}`))
		case 2:
			_, _ = writer.Write([]byte(
				`{"choices":[{"message":{"tool_calls":[{"id":"c1","type":"function",` +
					`"function":{"name":"probe__look","arguments":"{}"}}]}}]}`,
			))
		default:
			last := body.Messages[len(body.Messages)-1]
			if last.Role == "tool" && last.ToolCallID == "c1" && last.Content == withdrawnToolNotice {
				refusalSeen.Store(true)
			}
			_, _ = writer.Write([]byte(`{"choices":[{"message":{"content":"Request received."}}]}`))
		}
	}))
	defer server.Close()

	calls := &atomic.Int32{}
	client := ProxyClient{
		BaseURL:       server.URL,
		Model:         "selected-model",
		AuditRole:     "community",
		ResponseStyle: "neutral",
		Tools:         countingOneTool{calls: calls},
		HTTPClient:    &http.Client{Timeout: 5 * time.Second},
	}
	got, err := client.Complete(context.Background(), TurnPrompt{System: "s", Message: "u"}, "request")
	if err != nil {
		t.Fatalf("Complete: %v", err)
	}
	if got.Content != "Request received." {
		t.Fatalf("completion = %q", got.Content)
	}
	if !refusalSeen.Load() {
		t.Fatal("the refused call got no tool reply naming the withdrawal")
	}
	if calls.Load() != 0 {
		t.Fatalf("a withdrawn round ran the tool %d times", calls.Load())
	}
}

// countingOneTool is alwaysOneTool with a call counter.
type countingOneTool struct {
	alwaysOneTool
	calls *atomic.Int32
}

func (c countingOneTool) Open(context.Context) (ToolSession, error) { return c, nil }

func (c countingOneTool) Call(ctx context.Context, name string, args map[string]any) (ToolResult, error) {
	c.calls.Add(1)
	return c.alwaysOneTool.Call(ctx, name, args)
}
