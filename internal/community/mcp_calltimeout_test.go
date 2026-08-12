package community

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/modelcontextprotocol/go-sdk/mcp"
)

// hangingToolServer publishes one tool that never answers on its own, so only
// a caller-side bound can end the call.
func hangingToolServer(t *testing.T) string {
	t.Helper()
	server := mcp.NewServer(&mcp.Implementation{Name: "hang-test", Version: "1"}, nil)
	mcp.AddTool(
		server,
		&mcp.Tool{Name: "wait_forever", Description: "Never returns on its own."},
		func(ctx context.Context, _ *mcp.CallToolRequest, _ struct{}) (*mcp.CallToolResult, any, error) {
			<-ctx.Done()
			return nil, nil, ctx.Err()
		},
	)
	handler := httptest.NewServer(mcp.NewStreamableHTTPHandler(
		func(*http.Request) *mcp.Server { return server },
		&mcp.StreamableHTTPOptions{JSONResponse: true},
	))
	t.Cleanup(handler.Close)
	return handler.URL
}

// A hung tool used to run until the turn budget expired, which left nothing to
// report the failure with. See coilyco-gaming/sirens-echo#141.
func TestToolCallFailsOnItsOwnBoundNotTheTurnBudget(t *testing.T) {
	t.Parallel()
	provider := &MCPProvider{
		Servers:     []MCPServerDefinition{{Name: "hang", URL: hangingToolServer(t)}},
		HTTPClient:  &http.Client{Timeout: 30 * time.Second},
		CallTimeout: 150 * time.Millisecond,
	}
	t.Cleanup(func() { _ = provider.Close() })

	session, err := provider.Open(context.Background())
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	// The turn budget is generous, so anything that ends the call quickly can
	// only be the call's own bound.
	turnCtx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()

	started := time.Now()
	_, err = session.Call(turnCtx, "hang__wait_forever", nil)
	elapsed := time.Since(started)

	if err == nil {
		t.Fatal("Call returned no error for a tool that never answers")
	}
	if elapsed > 5*time.Second {
		t.Errorf("Call took %s, so it waited on the turn budget rather than its own bound", elapsed)
	}
	if turnCtx.Err() != nil {
		t.Errorf("turn context ended too, so the turn had no budget left to report with")
	}
}

// An unset bound must not mean an instant deadline, which a zero duration
// passed to WithTimeout would produce.
func TestZeroCallTimeoutFallsBackToTheDefault(t *testing.T) {
	t.Parallel()
	provider := &MCPProvider{}
	if got := provider.callTimeout(); got != defaultCallTimeout {
		t.Errorf("callTimeout = %s, want %s", got, defaultCallTimeout)
	}
	session := &mcpToolSession{registered: map[string]registeredMCPTool{}}
	if _, err := session.Call(context.Background(), "absent", nil); err == nil {
		t.Error("Call accepted an unregistered tool")
	}
}
