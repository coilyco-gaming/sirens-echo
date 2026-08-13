package community

import (
	"bytes"
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"
	"time"

	"github.com/modelcontextprotocol/go-sdk/mcp"
)

// The hour is only safe because a server that has never listed is retried on
// its own trigger. This is that trigger. See sirens-echo#163.
func TestAServerThatNeverListedRetriesRegardlessOfTheInterval(t *testing.T) {
	t.Parallel()
	entry := &supervisedServer{}
	if !entry.needsTools(time.Hour, time.Now()) {
		t.Fatal("a server with no tools waited out the interval, so a failed " +
			"listing would persist for the life of the pod")
	}
	// A server that did list is governed by the interval, which is the whole
	// point of raising it.
	entry.tools = []*mcp.Tool{}
	entry.refreshed = time.Now()
	if entry.needsTools(time.Hour, time.Now()) {
		t.Error("a freshly listed server re-listed inside the interval")
	}
}

func countingRosterServer(t *testing.T, lists *atomic.Int32) string {
	t.Helper()
	server := mcp.NewServer(&mcp.Implementation{Name: "roster-test", Version: "1"}, nil)
	mcp.AddTool(
		server,
		&mcp.Tool{Name: "status", Description: "fixture tool"},
		func(context.Context, *mcp.CallToolRequest, struct{}) (
			*mcp.CallToolResult, any, error,
		) {
			return &mcp.CallToolResult{
				Content: []mcp.Content{&mcp.TextContent{Text: "ok"}},
			}, nil, nil
		},
	)
	handler := mcp.NewStreamableHTTPHandler(
		func(*http.Request) *mcp.Server { return server },
		&mcp.StreamableHTTPOptions{JSONResponse: true},
	)
	httpServer := httptest.NewServer(http.HandlerFunc(func(
		writer http.ResponseWriter,
		request *http.Request,
	) {
		if request.Method == http.MethodPost {
			body, _ := io.ReadAll(request.Body)
			request.Body = io.NopCloser(bytes.NewReader(body))
			if bytes.Contains(body, []byte(`"method":"tools/list"`)) {
				lists.Add(1)
			}
		}
		handler.ServeHTTP(writer, request)
	}))
	t.Cleanup(httpServer.Close)
	return httpServer.URL
}

// An hour of staleness is only tolerable if something can end it early, and the
// agent is what notices a tool it expected is missing.
func TestRefreshRelistsWithoutWaitingOutTheInterval(t *testing.T) {
	t.Parallel()
	var lists atomic.Int32
	url := countingRosterServer(t, &lists)

	provider := &MCPProvider{
		Servers:         []MCPServerDefinition{{Name: "roster", URL: url}},
		RefreshInterval: time.Hour,
	}
	t.Cleanup(func() { _ = provider.Close() })

	openTurn := func() {
		t.Helper()
		session, err := provider.Open(context.Background())
		if err != nil {
			t.Fatalf("Open: %v", err)
		}
		if err := session.Close(); err != nil {
			t.Fatalf("Close: %v", err)
		}
	}

	openTurn()
	openTurn()
	if got := lists.Load(); got != 1 {
		t.Fatalf("listings across two turns inside the interval = %d, want 1", got)
	}

	if marked := provider.Refresh(); marked != 1 {
		t.Fatalf("Refresh marked %d servers, want 1", marked)
	}
	openTurn()
	if got := lists.Load(); got != 2 {
		t.Errorf("listings after a refresh = %d, want 2", got)
	}
}

// Refresh dials nothing itself, which is why it needs no rate limit of its own:
// repeated calls collapse into the next turn's single listing.
func TestRefreshIsIdempotentUntilTheNextTurn(t *testing.T) {
	t.Parallel()
	var lists atomic.Int32
	url := countingRosterServer(t, &lists)

	provider := &MCPProvider{
		Servers:         []MCPServerDefinition{{Name: "roster", URL: url}},
		RefreshInterval: time.Hour,
	}
	t.Cleanup(func() { _ = provider.Close() })

	session, err := provider.Open(context.Background())
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	if err := session.Close(); err != nil {
		t.Fatalf("Close: %v", err)
	}

	for call := 0; call < 20; call++ {
		provider.Refresh()
	}
	if got := lists.Load(); got != 1 {
		t.Fatalf("twenty refreshes listed %d times before any turn ran, want 1", got)
	}

	session, err = provider.Open(context.Background())
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	if err := session.Close(); err != nil {
		t.Fatalf("Close: %v", err)
	}
	if got := lists.Load(); got != 2 {
		t.Errorf("listings after twenty refreshes and one turn = %d, want 2", got)
	}
}
