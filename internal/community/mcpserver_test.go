package community

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/modelcontextprotocol/go-sdk/mcp"
)

// fixedCompletionClient answers every request, because an MCP turn generates
// its own request id and cannot be keyed by one.
type fixedCompletionClient struct{ reply string }

func (f fixedCompletionClient) Complete(
	context.Context,
	string, string, string,
) (CompletionResult, error) {
	return CompletionResult{Content: f.reply}, nil
}

func mcpServedAgent(t *testing.T, reply string) *mcp.ClientSession {
	t.Helper()
	agent := &Agent{
		cfg: Config{
			InstanceName: "sirens-echo",
			Definition:   Definition{Identity: "Sirens Echo", MaxContextMessages: 12},
		},
		completions:  fixedCompletionClient{reply: reply},
		systemPrompt: "neutral model policy and local knowledge",
		telemetry:    telemetryOrNoop(nil),
		slots:        make(chan struct{}, 1),
	}
	server := httptest.NewServer(agent.HTTPHandler())
	t.Cleanup(server.Close)

	client := mcp.NewClient(&mcp.Implementation{Name: "fleet-client", Version: "1"}, nil)
	session, err := client.Connect(context.Background(), &mcp.StreamableClientTransport{
		Endpoint:             server.URL + mcpServerPath,
		DisableStandaloneSSE: true,
	}, nil)
	if err != nil {
		t.Fatalf("connect to Echo over MCP: %v", err)
	}
	t.Cleanup(func() { _ = session.Close() })
	return session
}

func TestEchoServesItsTurnOverMCP(t *testing.T) {
	t.Parallel()
	session := mcpServedAgent(t, "Echo is ready.")

	tools, err := session.ListTools(context.Background(), nil)
	if err != nil {
		t.Fatalf("ListTools: %v", err)
	}
	if len(tools.Tools) != 1 || tools.Tools[0].Name != "turn" {
		t.Fatalf("tools = %#v, want one turn tool", tools.Tools)
	}

	result, err := session.CallTool(context.Background(), &mcp.CallToolParams{
		Name:      "turn",
		Arguments: map[string]any{"author": "tester", "content": "Are you ready?"},
	})
	if err != nil {
		t.Fatalf("CallTool: %v", err)
	}
	if result.IsError {
		t.Fatalf("result = %#v, want a successful turn", result)
	}
	if text := callResultText(result); !strings.Contains(text, "Echo is ready.") {
		t.Fatalf("reply = %q", text)
	}
}

func TestEchoMCPTurnReportsCallerErrorsAsToolData(t *testing.T) {
	t.Parallel()
	session := mcpServedAgent(t, "Echo is ready.")

	// A caller-fixable problem is data the calling model can see and correct,
	// not a protocol error.
	result, err := session.CallTool(context.Background(), &mcp.CallToolParams{
		Name:      "turn",
		Arguments: map[string]any{"author": "tester", "content": "   "},
	})
	if err != nil {
		t.Fatalf("CallTool: %v", err)
	}
	if !result.IsError {
		t.Fatal("empty content was accepted")
	}
	if text := callResultText(result); !strings.Contains(text, "content is required") {
		t.Fatalf("error text = %q", text)
	}
}

func TestMCPPrincipalPrefersTheSharedCallerHeader(t *testing.T) {
	t.Parallel()
	header := http.Header{}
	header.Set("X-Sirens-Caller", "release-bot")
	withHeader := &mcp.CallToolRequest{Extra: &mcp.RequestExtra{Header: header}}
	if got := mcpPrincipal(withHeader); got != "mcp:release-bot" {
		t.Fatalf("principal = %q", got)
	}
	// No header and no session leaves one shared budget rather than none.
	if got := mcpPrincipal(&mcp.CallToolRequest{}); got != "mcp:anonymous" {
		t.Fatalf("principal = %q", got)
	}
}

func callResultText(result *mcp.CallToolResult) string {
	parts := make([]string, 0, len(result.Content))
	for _, content := range result.Content {
		if text, ok := content.(*mcp.TextContent); ok {
			parts = append(parts, text.Text)
		}
	}
	return strings.Join(parts, "\n")
}
