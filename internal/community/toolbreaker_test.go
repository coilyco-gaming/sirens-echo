package community

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/modelcontextprotocol/go-sdk/mcp"
)

// A tool that reported its own failure is not called again that turn, and the
// model is told rather than denied. See sirens-echo#943.

// brokenTool registers a name whose session would panic if it were dispatched,
// so surviving the call is the proof that no call was made.
func brokenTool(name, recorded string, arguments map[string]any) *mcpToolSession {
	return &mcpToolSession{
		registered: map[string]registeredMCPTool{
			name: {serverName: "playwright", toolName: "browser_navigate"},
		},
		failed: map[string]string{callKey(name, arguments): recorded},
	}
}

func TestASecondCallToAFailedToolIsNotMade(t *testing.T) {
	t.Parallel()
	const recorded = "mcp-beaver: upstream MCP session is closed"
	arguments := map[string]any{"url": "x"}
	session := brokenTool("playwright__browser_navigate", recorded, arguments)

	result, err := session.Call(
		context.Background(), "playwright__browser_navigate", arguments,
	)
	if err != nil {
		t.Fatalf("the breaker returned a transport error, which would end the turn: %v", err)
	}
	if !result.IsError {
		t.Fatal("the replayed failure is not an error, so the model reads it as a result")
	}
	if !strings.Contains(result.Text, recorded) {
		t.Errorf("the reply %q drops what the tool actually said", result.Text)
	}
	// The model has to be able to tell a refusal from a fresh failure, or it
	// cannot decide to answer the half that needs no tool.
	if !strings.Contains(result.Text, "not called again") {
		t.Errorf("the reply %q does not say the call was skipped", result.Text)
	}
	if outcomeOf(result) != ToolOutcomeFailed {
		t.Errorf("outcome = %q, want the failure the disclosure footer renders", outcomeOf(result))
	}
}

// The breaker is per tool. One dead server must not mute the rest of a roster,
// which is the failure a round cap would have introduced and Kai declined.
func TestTheBreakerStopsOneToolRatherThanTheRoster(t *testing.T) {
	t.Parallel()
	dead := map[string]any{"url": "x"}
	session := brokenTool("playwright__browser_navigate", "closed", dead)
	session.registered["forgejo__list_issue"] = registeredMCPTool{
		serverName: "forgejo", toolName: "list_issue",
	}
	if _, spent := session.alreadyFailed(callKey("forgejo__list_issue", nil)); spent {
		t.Fatal("a healthy tool was broken by another tool's failure")
	}
	if _, spent := session.alreadyFailed(callKey("playwright__browser_navigate", dead)); !spent {
		t.Fatal("the failed call is not recorded")
	}
}

// The observed defect was byte-identical repeats. A model that corrects its
// arguments is doing the right thing and must not be refused for it.
func TestACorrectedRetryIsNotRefused(t *testing.T) {
	t.Parallel()
	session := brokenTool(
		"forgejo__list_issue", "unknown field", map[string]any{"q": "typo"},
	)
	if _, spent := session.alreadyFailed(
		callKey("forgejo__list_issue", map[string]any{"q": "fixed"}),
	); spent {
		t.Fatal("a corrected retry was refused, so the model cannot recover from its own mistake")
	}
	if _, spent := session.alreadyFailed(
		callKey("forgejo__list_issue", map[string]any{"q": "typo"}),
	); !spent {
		t.Fatal("the identical repeat is not blocked, which is the call that was wasted")
	}
}

// The replay describes the call that happened, so a later error must not
// overwrite the first and describe a call that never ran.
func TestTheRecordedFailureIsTheFirstOne(t *testing.T) {
	t.Parallel()
	session := &mcpToolSession{}
	key := callKey("t", nil)
	session.recordFailure(key, "first")
	session.recordFailure(key, "second")
	if recorded, _ := session.alreadyFailed(key); recorded != "first" {
		t.Errorf("recorded %q, want the failure that was actually returned", recorded)
	}
}

// A tool that never failed is untouched, or the breaker would answer every call
// from an empty record.
func TestAToolThatNeverFailedIsNotBroken(t *testing.T) {
	t.Parallel()
	session := &mcpToolSession{}
	if _, spent := session.alreadyFailed(callKey("t", nil)); spent {
		t.Fatal("a tool with no record reads as broken")
	}
	// A failure with no text still stops the second call, and still says so.
	session.recordFailure(callKey("t", nil), "")
	if notice := repeatedFailureNotice(""); !strings.Contains(notice, "not called again") {
		t.Errorf("an empty record renders %q", notice)
	}
}

// toolBearingServer advertises one tool, so a registered count means something.
// liveToolServer advertises none, which is why it cannot answer this.
func toolBearingServer(t *testing.T) string {
	t.Helper()
	server := mcp.NewServer(&mcp.Implementation{Name: "eco-test", Version: "1"}, nil)
	mcp.AddTool(
		server,
		&mcp.Tool{Name: "get_market", Description: "fixture tool"},
		func(context.Context, *mcp.CallToolRequest, struct{}) (
			*mcp.CallToolResult, any, error,
		) {
			return &mcp.CallToolResult{
				Content: []mcp.Content{&mcp.TextContent{Text: "ok"}},
			}, nil, nil
		},
	)
	httpServer := httptest.NewServer(mcp.NewStreamableHTTPHandler(
		func(*http.Request) *mcp.Server { return server },
		&mcp.StreamableHTTPOptions{JSONResponse: true},
	))
	t.Cleanup(httpServer.Close)
	return httpServer.URL
}

// The instrument #943 asks for: a warm roster and a dead one both report
// cached: true with reached: 0, and only these two fields tell them apart.
func TestAWarmRosterIsDistinguishableFromADeadOne(t *testing.T) {
	t.Parallel()
	provider := &MCPProvider{
		Servers:    []MCPServerDefinition{{Name: "eco", URL: toolBearingServer(t)}},
		HTTPClient: &http.Client{Timeout: 5 * time.Second},
	}
	t.Cleanup(func() { _ = provider.Close() })

	listingSpanAttributes(t, provider)
	warm := listingSpanAttributes(t, provider)
	if warm["mcp.tools.cached"] != "true" || warm["mcp.tools.reached"] != "0" {
		t.Fatalf("the second listing was not a cache hit: %v", warm)
	}
	if warm["mcp.tools.unavailable"] != "0" {
		t.Errorf("a warm roster reports %q servers unavailable", warm["mcp.tools.unavailable"])
	}
	// The question #943 could not answer from telemetry: is the model getting
	// tools. A warm hit serves the roster it already read.
	if warm["mcp.tools.registered"] == "0" {
		t.Error("a warm roster registered no tools, which is the collapse 943 measured")
	}

	dead := &MCPProvider{
		Servers:    []MCPServerDefinition{{Name: "eco", URL: unreachableToolServer(t)}},
		HTTPClient: &http.Client{Timeout: time.Second},
	}
	t.Cleanup(func() { _ = dead.Close() })
	outage := listingSpanAttributes(t, dead)
	if outage["mcp.tools.unavailable"] != "1" {
		t.Errorf("an unreachable server is not counted unavailable: %v", outage)
	}
	if outage["mcp.tools.registered"] != "0" {
		t.Errorf("an unreachable server registered tools: %v", outage)
	}
}
