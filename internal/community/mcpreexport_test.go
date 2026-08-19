package community

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"slices"
	"strings"
	"testing"

	"github.com/modelcontextprotocol/go-sdk/mcp"
)

func reexportFixture() FixtureProvider {
	return FixtureProvider{Pack: FixturePack{Tools: []FixtureTool{
		{
			Name:        "forgejo__list_issue",
			Server:      "forgejo",
			Description: "List issues in a repository.",
			Result:      "one issue",
		},
		{
			Name:        "moxn__find",
			Server:      "moxn",
			Description: "List documents in a filesystem.",
			Result:      "one document",
		},
	}}}
}

// reexportAgent stands up the lane over MCP with the roster fixture attached.
// configured is the deployment's token, empty meaning it trusts nobody.
// presented is what the client sends, which is separate on purpose: the gate
// only means something when a client can hold the wrong answer.
func reexportAgent(t *testing.T, enabled bool, configured, presented string) *mcp.ClientSession {
	t.Helper()
	agent := &Agent{
		cfg: Config{
			InstanceName:   "sirens-dowel",
			Definition:     Definition{Identity: "Sirens Dowel", MaxContextMessages: 12},
			MCPReexport:    enabled,
			HTTPTrustToken: configured,
		},
		completions:  fixedCompletionClient{reply: "a reply"},
		systemPrompt: "neutral model policy",
		telemetry:    telemetryOrNoop(nil),
		slots:        make(chan struct{}, 1),
	}
	agent.reexport.provider = reexportFixture()

	server := httptest.NewServer(agent.HTTPHandler())
	t.Cleanup(server.Close)

	transport := &mcp.StreamableClientTransport{
		Endpoint:             server.URL + mcpServerPath,
		DisableStandaloneSSE: true,
	}
	if presented != "" {
		transport.HTTPClient = bearerClient(presented)
	}
	client := mcp.NewClient(&mcp.Implementation{Name: "fleet-client", Version: "1"}, nil)
	session, err := client.Connect(context.Background(), transport, nil)
	if err != nil {
		t.Fatalf("connect over MCP: %v", err)
	}
	t.Cleanup(func() { _ = session.Close() })
	return session
}

func listedToolNames(t *testing.T, session *mcp.ClientSession) []string {
	t.Helper()
	result, err := session.ListTools(context.Background(), nil)
	if err != nil {
		t.Fatalf("list tools: %v", err)
	}
	names := make([]string, 0, len(result.Tools))
	for _, tool := range result.Tools {
		names = append(names, tool.Name)
	}
	slices.Sort(names)
	return names
}

// The default is the surface every lane serves today, so opting out has to be
// byte-for-byte what shipped before this existed.
func TestReexportDisabledServesTurnAlone(t *testing.T) {
	session := reexportAgent(t, false, "secret", "secret")
	names := listedToolNames(t, session)
	if !slices.Equal(names, []string{"turn"}) {
		t.Fatalf("disabled lane offers %v, want [turn]", names)
	}
}

func TestReexportEnabledOffersRosterBesideTurn(t *testing.T) {
	session := reexportAgent(t, true, "secret", "secret")
	names := listedToolNames(t, session)
	want := []string{"forgejo__list_issue", "moxn__find", "turn"}
	if !slices.Equal(names, want) {
		t.Fatalf("enabled lane offers %v, want %v", names, want)
	}
}

// The names carry their server, so two servers offering find cannot collide
// into one entry a caller cannot address.
func TestReexportedNamesCarryTheirServer(t *testing.T) {
	session := reexportAgent(t, true, "secret", "secret")
	for _, name := range listedToolNames(t, session) {
		if name == "turn" {
			continue
		}
		if !strings.Contains(name, "__") {
			t.Errorf("re-exported tool %q is not namespaced by server", name)
		}
	}
}

// The point of the gate. A caller past this reaches a server directly without
// the turn pipeline's checks, so an unauthenticated one must be refused.
func TestReexportedCallRefusesAnUntrustedCaller(t *testing.T) {
	session := reexportAgent(t, true, "secret", "")
	result, err := session.CallTool(context.Background(), &mcp.CallToolParams{
		Name:      "forgejo__list_issue",
		Arguments: map[string]any{},
	})
	if err != nil {
		t.Fatalf("call tool: %v", err)
	}
	if !result.IsError {
		t.Fatal("untrusted call succeeded, want refusal")
	}
	if !strings.Contains(toolText(result), "deployment token") {
		t.Errorf("refusal does not name the token: %q", toolText(result))
	}
}

// An empty configured token trusts nobody, so enabling re-export without one
// offers tools that refuse every caller rather than tools that admit everyone.
func TestReexportWithNoTokenRefusesEveryone(t *testing.T) {
	session := reexportAgent(t, true, "", "")
	result, err := session.CallTool(context.Background(), &mcp.CallToolParams{
		Name:      "moxn__find",
		Arguments: map[string]any{},
	})
	if err != nil {
		t.Fatalf("call tool: %v", err)
	}
	if !result.IsError {
		t.Fatal("call succeeded with no configured token, want refusal")
	}
}

func TestReexportedCallReachesTheServerForATrustedCaller(t *testing.T) {
	session := reexportAgent(t, true, "secret", "secret")
	result, err := session.CallTool(context.Background(), &mcp.CallToolParams{
		Name:      "moxn__find",
		Arguments: map[string]any{},
	})
	if err != nil {
		t.Fatalf("call tool: %v", err)
	}
	if result.IsError {
		t.Fatalf("trusted call refused: %q", toolText(result))
	}
	if !strings.Contains(toolText(result), "one document") {
		t.Errorf("result %q does not carry the server's answer", toolText(result))
	}
}

// turn keeps working when re-export is on, because opting in adds a surface
// rather than replacing the one every existing client uses.
func TestTurnStillAnswersWhenReexportIsOn(t *testing.T) {
	session := reexportAgent(t, true, "secret", "secret")
	result, err := session.CallTool(context.Background(), &mcp.CallToolParams{
		Name:      "turn",
		Arguments: map[string]any{"content": "hello"},
	})
	if err != nil {
		t.Fatalf("call turn: %v", err)
	}
	if result.IsError {
		t.Fatalf("turn refused with re-export on: %q", toolText(result))
	}
}

// #943 recorded the roster collapsing to zero and back. A client whose tool
// list empties underneath it is worse than one holding a slightly stale list,
// so a failed refresh keeps what it had.
func TestSnapshotKeepsThePreviousRosterWhenDiscoveryFails(t *testing.T) {
	agent := &Agent{cfg: Config{MCPReexport: true}, telemetry: telemetryOrNoop(nil)}
	agent.reexport.provider = reexportFixture()
	first := agent.reexportSnapshot(context.Background())
	if len(first) != 2 {
		t.Fatalf("first snapshot has %d tools, want 2", len(first))
	}

	agent.reexport.provider = failingProvider{}
	agent.reexport.fetched = agent.reexport.fetched.Add(-2 * reexportRefreshInterval)
	second := agent.reexportSnapshot(context.Background())
	if len(second) != 2 {
		t.Fatalf("snapshot after a failed refresh has %d tools, want the previous 2", len(second))
	}
}

// toolText flattens a result's text content, so an assertion reads the answer
// rather than the envelope.
func toolText(result *mcp.CallToolResult) string {
	if result == nil {
		return ""
	}
	var parts []string
	for _, content := range result.Content {
		if text, ok := content.(*mcp.TextContent); ok {
			parts = append(parts, text.Text)
		}
	}
	return strings.Join(parts, "\n")
}

// bearerClient presents the deployment token on every request, which is how a
// trusted fleet client reaches the re-exported surface.
func bearerClient(token string) *http.Client {
	return &http.Client{Transport: bearerTransport{token: token}}
}

type bearerTransport struct{ token string }

func (b bearerTransport) RoundTrip(request *http.Request) (*http.Response, error) {
	clone := request.Clone(request.Context())
	clone.Header.Set(httpTrustHeader, httpTrustScheme+b.token)
	return http.DefaultTransport.RoundTrip(clone)
}

// failingProvider stands in for a roster that stopped answering.
type failingProvider struct{}

func (failingProvider) Open(context.Context) (ToolSession, error) {
	return nil, errRosterUnavailable
}

var errRosterUnavailable = errors.New("roster unavailable")

// A caller holding the wrong token is refused exactly as one holding none, so
// the gate is a comparison rather than a presence check.
func TestReexportedCallRefusesAWrongToken(t *testing.T) {
	session := reexportAgent(t, true, "secret", "not-the-secret")
	result, err := session.CallTool(context.Background(), &mcp.CallToolParams{
		Name:      "moxn__find",
		Arguments: map[string]any{},
	})
	if err != nil {
		t.Fatalf("call tool: %v", err)
	}
	if !result.IsError {
		t.Fatal("call with a wrong token succeeded, want refusal")
	}
}
