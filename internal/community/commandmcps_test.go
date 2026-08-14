package community

import (
	"strings"
	"testing"

	"github.com/bwmarrin/discordgo"
)

// The roster is the configured set, so a server that answered with nothing is
// reported rather than dropped. Silence there reads as a smaller deployment.
func TestRenderMCPRosterReportsEveryConfiguredServer(t *testing.T) {
	t.Parallel()
	configured := []MCPServerDefinition{{Name: "eco"}, {Name: "forgejo"}, {Name: "quiet"}}
	tools := []ToolDefinition{
		{Server: "eco", Name: "eco_get_market", Original: "get_market"},
		{Server: "eco", Name: "eco_get_stores", Original: "get_stores"},
		{Server: "forgejo", Name: "forgejo_list_issue", Original: "list_issue"},
	}
	rendered := renderMCPRoster(configured, tools, nil)
	for _, expected := range []string{
		"eco (2): get_market, get_stores",
		"forgejo (1): list_issue",
		"quiet: no tools",
	} {
		if !strings.Contains(rendered, expected) {
			t.Fatalf("roster missing %q:\n%s", expected, rendered)
		}
	}
}

// A server that did not answer is a different fact from one with no tools, and
// an operator reading this needs to know which happened.
func TestRenderMCPRosterSeparatesUnavailableFromEmpty(t *testing.T) {
	t.Parallel()
	rendered := renderMCPRoster(
		[]MCPServerDefinition{{Name: "steam"}, {Name: "forgejo"}},
		nil,
		[]string{"steam"},
	)
	if !strings.Contains(rendered, "steam: did not answer this turn") {
		t.Fatalf("unavailable server not named:\n%s", rendered)
	}
	if !strings.Contains(rendered, "forgejo: no tools") {
		t.Fatalf("empty server not distinguished:\n%s", rendered)
	}
}

// The URL is an in-cluster address the deployment owns. Nothing renders it, and
// this is the test that says so rather than the reviewer noticing.
func TestRenderMCPRosterNeverRendersAnAddress(t *testing.T) {
	t.Parallel()
	rendered := renderMCPRoster(
		[]MCPServerDefinition{{
			Name:      "forgejo",
			URL:       "http://sirens-echo-forgejo-mcp:8080/mcp",
			Transport: "streamable",
			Env:       map[string]string{"TOKEN": "secret"},
		}},
		[]ToolDefinition{{Server: "forgejo", Original: "list_issue"}},
		nil,
	)
	for _, forbidden := range []string{"http://", "8080", "sirens-echo-forgejo-mcp", "secret"} {
		if strings.Contains(rendered, forbidden) {
			t.Fatalf("roster leaked %q:\n%s", forbidden, rendered)
		}
	}
}

// A roster past the reply bound is cut with a line saying so, because a
// silently short list is indistinguishable from a short roster.
func TestRenderMCPRosterSaysWhenItTruncated(t *testing.T) {
	t.Parallel()
	configured := make([]MCPServerDefinition, 0, 60)
	tools := make([]ToolDefinition, 0, 600)
	for index := 0; index < 60; index++ {
		name := strings.Repeat("s", 20) + string(rune('a'+index%26))
		configured = append(configured, MCPServerDefinition{Name: name})
		for tool := 0; tool < 10; tool++ {
			tools = append(tools, ToolDefinition{Server: name, Original: strings.Repeat("t", 15)})
		}
	}
	rendered := renderMCPRoster(configured, tools, nil)
	if len(rendered) > 1990 {
		t.Fatalf("rendered %d runes, past the interaction bound", len(rendered))
	}
	if !strings.Contains(rendered, "not shown") {
		t.Fatalf("truncation was silent:\n%s", rendered[:200])
	}
}

func TestRenderMCPRosterHandlesNoServers(t *testing.T) {
	t.Parallel()
	if got := renderMCPRoster(nil, nil, nil); !strings.Contains(got, "no MCP server") {
		t.Fatalf("empty roster = %q", got)
	}
}

// Introspection answers the caller, not the channel. The declaration carries
// that, so the handler cannot forget it for one command.
func TestMCPSCommandIsDeclaredEphemeralAndJobless(t *testing.T) {
	t.Parallel()
	command, declared := LookupCommand("mcps")
	if !declared {
		t.Fatal("mcps is not declared")
	}
	if !command.Ephemeral {
		t.Fatal("mcps answers the whole channel")
	}
	if command.Kind != "" {
		t.Fatalf("mcps submits job kind %q", command.Kind)
	}
	if len(command.Parameters) != 0 {
		t.Fatalf("mcps declares %d parameters", len(command.Parameters))
	}
	if err := command.Validate(); err != nil {
		t.Fatalf("Validate: %v", err)
	}
}

func TestEphemeralFlagMapsToDiscord(t *testing.T) {
	t.Parallel()
	if ephemeralFlag(true) != discordgo.MessageFlagsEphemeral {
		t.Fatal("ephemeral command would answer the channel")
	}
	if ephemeralFlag(false) != 0 {
		t.Fatal("an ordinary command gained a flag")
	}
}
