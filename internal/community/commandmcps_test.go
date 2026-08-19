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
	command, declared := LookupCommand("mcps", nil)
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

// /mcps is the index and /mcp is the detail. The split exists because a tool's
// own description never fits an index bounded by one reply.

func TestTheSingularNamesWhatEachToolDoes(t *testing.T) {
	t.Parallel()
	tools := []ToolDefinition{
		{Server: "eco", Name: "eco_get_market", Original: "get_market",
			Description: "Current buy and sell orders across every store."},
		{Server: "eco", Name: "eco_get_stores", Original: "get_stores",
			Description: "Every store, its owner, and its balance."},
		{Server: "forgejo", Name: "forgejo_list_issue", Original: "list_issue",
			Description: "Issues in this repository."},
	}
	rendered := renderMCPServer("eco", tools, nil)
	for _, want := range []string{"get_market", "Current buy and sell orders", "get_stores"} {
		if !strings.Contains(rendered, want) {
			t.Fatalf("rendered %q is missing %q", rendered, want)
		}
	}
	// Another server's tools are not this server's, and an index that leaked
	// them would make the count wrong as well as the list.
	if strings.Contains(rendered, "list_issue") {
		t.Fatalf("rendered %q carried another server's tool", rendered)
	}
	if !strings.Contains(rendered, "eco (2)") {
		t.Fatalf("rendered %q does not count the server's own tools", rendered)
	}
}

// A server's description is the server's own text, so it is bounded before it
// reaches a reply the way every other untrusted string is.
func TestASingleToolCannotSpendTheWholeReply(t *testing.T) {
	restoreKnobs(t)
	applyKnobs(fixedLookup(map[string]string{"SIRENS_ECHO_MCP_TOOL_SUMMARY_RUNES": "20"}))
	rendered := renderMCPServer("eco", []ToolDefinition{{
		Server: "eco", Original: "get_market", Description: strings.Repeat("verbose ", 200),
	}}, nil)
	if len(rendered) > 200 {
		t.Fatalf("one tool rendered %d characters", len(rendered))
	}
}

// A description arrives as whatever the server wrote, newlines included, and a
// multi-line one would break the one-line-per-tool shape.
func TestAToolDescriptionIsFlattenedToOneLine(t *testing.T) {
	t.Parallel()
	rendered := renderMCPServer("eco", []ToolDefinition{{
		Server: "eco", Original: "get_market", Description: "Orders\nacross\n\nevery store.",
	}}, nil)
	if strings.Count(rendered, "\n") != 1 {
		t.Fatalf("rendered %q spans more than the header and one tool", rendered)
	}
}

func TestTheSingularSeparatesASilentServerFromAnEmptyOne(t *testing.T) {
	t.Parallel()
	if got := renderMCPServer("eco", nil, []string{"eco"}); !strings.Contains(got, "did not answer") {
		t.Fatalf("an unreachable server rendered %q", got)
	}
	if got := renderMCPServer("eco", nil, nil); !strings.Contains(got, "no tools") {
		t.Fatalf("a server answering with nothing rendered %q", got)
	}
}

// The argument names a thing and the roster is fixed at image build, so it
// carries choices, which is the tightest bound available.
func TestTheSingularClosesItsArgumentToTheConfiguredRoster(t *testing.T) {
	t.Parallel()
	servers := []string{"eco", "forgejo"}
	command, declared := LookupCommand("mcp", servers)
	if !declared {
		t.Fatal("mcp is not declared against a configured roster")
	}
	if !command.Ephemeral {
		t.Fatal("mcp answers the whole channel")
	}
	if command.Kind != "" {
		t.Fatalf("mcp submits job kind %q", command.Kind)
	}
	if len(command.Parameters) != 1 {
		t.Fatalf("mcp declares %d parameters, want the server", len(command.Parameters))
	}
	parameter := command.Parameters[0]
	if !parameter.Required {
		t.Fatal("mcp's server argument is optional, so the command has no subject")
	}
	if len(parameter.Choices) != len(servers) {
		t.Fatalf("choices are %v, want the configured roster", parameter.Choices)
	}
	if _, err := command.BindArguments(map[string]string{"server": "eco"}); err != nil {
		t.Fatalf("a configured server was refused: %v", err)
	}
	if _, err := command.BindArguments(map[string]string{"server": "elsewhere"}); err == nil {
		t.Fatal("a server outside the roster was accepted")
	}
}

// A deployment with no MCP publishes no command rather than one whose only
// argument names nothing a caller could pick.
func TestNoRosterPublishesNoSingular(t *testing.T) {
	t.Parallel()
	if _, declared := LookupCommand("mcp", nil); declared {
		t.Fatal("mcp is published with no server to describe")
	}
	if _, declared := LookupCommand("mcps", nil); !declared {
		t.Fatal("mcps stopped being published")
	}
	rendered, err := discordCommands(nil)
	if err != nil {
		t.Fatalf("discordCommands: %v", err)
	}
	for _, command := range rendered {
		if command.Name == "mcp" {
			t.Fatal("mcp reached Discord with no roster behind it")
		}
	}
}

// The published set has to carry the choices, or Discord advertises a free
// string and the picker stops helping.
func TestTheRegisteredSingularCarriesItsChoices(t *testing.T) {
	t.Parallel()
	rendered, err := discordCommands([]string{"eco", "forgejo"})
	if err != nil {
		t.Fatalf("discordCommands: %v", err)
	}
	for _, command := range rendered {
		if command.Name != "mcp" {
			continue
		}
		if len(command.Options) != 1 || len(command.Options[0].Choices) != 2 {
			t.Fatalf("mcp published options %+v", command.Options)
		}
		return
	}
	t.Fatal("mcp was not registered against a configured roster")
}
