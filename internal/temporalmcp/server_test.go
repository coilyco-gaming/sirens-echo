package temporalmcp

import (
	"context"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/modelcontextprotocol/go-sdk/mcp"
)

// Deny-by-absence is the whole enforcement here, so what this file guards is
// the absence rather than any behaviour of the three tools that exist.

func connect(t *testing.T) *mcp.ClientSession {
	t.Helper()
	handler := Handler(nil, Config{Namespace: "coilyco.gcdqf", Instance: "sirens-dowel"})
	server := httptest.NewServer(handler)
	t.Cleanup(server.Close)
	client := mcp.NewClient(&mcp.Implementation{Name: "test", Version: "1"}, nil)
	session, err := client.Connect(context.Background(), &mcp.StreamableClientTransport{
		Endpoint:             server.URL + Path,
		DisableStandaloneSSE: true,
	}, nil)
	if err != nil {
		t.Fatalf("connect: %v", err)
	}
	t.Cleanup(func() { _ = session.Close() })
	return session
}

func TestOnlyReadToolsAreServed(t *testing.T) {
	tools, err := connect(t).ListTools(context.Background(), nil)
	if err != nil {
		t.Fatalf("list tools: %v", err)
	}
	served := map[string]bool{}
	for _, tool := range tools.Tools {
		served[tool.Name] = true
		if tool.Annotations == nil || !tool.Annotations.ReadOnlyHint {
			t.Errorf("tool %q does not declare readOnlyHint", tool.Name)
		}
	}
	for _, want := range []string{"list_workflows", "describe_workflow", "get_workflow_history"} {
		if !served[want] {
			t.Errorf("tool %q is missing", want)
		}
	}
	if len(tools.Tools) != 3 {
		t.Fatalf("want exactly the three read tools, got %d: %v", len(tools.Tools), served)
	}
}

// The same screen mcp-beaver's lint-upstream applies to an allowlist, kept here
// so a mutating tool cannot be added without this failing first.
func TestNoMutatingVerbIsReachable(t *testing.T) {
	tools, err := connect(t).ListTools(context.Background(), nil)
	if err != nil {
		t.Fatalf("list tools: %v", err)
	}
	mutating := []string{
		"start", "signal", "cancel", "terminate", "delete", "create",
		"update", "reset", "pause", "unpause", "trigger", "batch",
	}
	for _, tool := range tools.Tools {
		for _, verb := range mutating {
			if strings.Contains(tool.Name, verb) {
				t.Errorf("tool %q names the mutating verb %q", tool.Name, verb)
			}
		}
	}
}

func TestDialRefusesAHalfFilledConnection(t *testing.T) {
	for name, cfg := range map[string]Config{
		"no host":      {Namespace: "coilyco.gcdqf"},
		"no namespace": {HostPort: "coilyco.gcdqf.tmprl.cloud:7233"},
	} {
		t.Run(name, func(t *testing.T) {
			if _, err := Dial(cfg); err == nil {
				t.Fatal("a half-filled connection was accepted")
			}
		})
	}
}
