package community

import (
	"os"
	"path/filepath"
	"testing"
)

func writeRoster(t *testing.T, body string) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "mcp.json")
	if err := os.WriteFile(path, []byte(body), 0o600); err != nil {
		t.Fatalf("write roster: %v", err)
	}
	return path
}

func TestLoadMCPRosterReadsTheSharedMCPServersShape(t *testing.T) {
	t.Setenv("ROSTER_TEST_FORGEJO", "http://forgejo-mcp:8080/mcp")
	// The shape mcporter, Claude Code, and Codex already use, including keys
	// Echo does not read. An unknown key is another tool's extension.
	path := writeRoster(t, `{
	  "imports": [],
	  "mcpServers": {
	    "eco": {"baseUrl": "https://eco-app.example/mcp", "description": "eco"},
	    "forgejo": {"url": "${ROSTER_TEST_FORGEJO}", "x-codex": {"a": 1}},
	    "local": {"command": "/usr/bin/mcp", "args": ["--flag"],
	              "env": {"SOME_TOKEN": "${ROSTER_TEST_FORGEJO}"}}
	  }
	}`)

	servers, err := LoadMCPRoster(path)
	if err != nil {
		t.Fatalf("LoadMCPRoster: %v", err)
	}
	if len(servers) != 3 {
		t.Fatalf("servers = %#v", servers)
	}
	// Sorted, because map order is random and the roster decides tool order.
	if servers[0].Name != "eco" || servers[1].Name != "forgejo" || servers[2].Name != "local" {
		t.Fatalf("order = %#v", servers)
	}
	if servers[0].ResolvedTransport() != MCPTransportStreamable {
		t.Fatalf("eco transport = %q", servers[0].ResolvedTransport())
	}
	// url is accepted alongside baseUrl, and ${VAR} resolves from the process.
	if servers[1].URL != "http://forgejo-mcp:8080/mcp" {
		t.Fatalf("forgejo url = %q", servers[1].URL)
	}
	if servers[2].ResolvedTransport() != MCPTransportStdio {
		t.Fatalf("local transport = %q", servers[2].ResolvedTransport())
	}
	if servers[2].Env["SOME_TOKEN"] != "http://forgejo-mcp:8080/mcp" {
		t.Fatalf("local env = %#v", servers[2].Env)
	}
}

func TestLoadMCPRosterRejectsAnUnresolvedVariable(t *testing.T) {
	t.Setenv("ROSTER_TEST_ABSENT", "")
	path := writeRoster(t, `{"mcpServers": {"eco": {"baseUrl": "${ROSTER_TEST_ABSENT}"}}}`)

	// An unset variable expands to empty, which must name the server rather
	// than connect to nothing.
	if _, err := LoadMCPRoster(path); err == nil {
		t.Fatal("an unresolved endpoint was accepted")
	}
}

func TestLoadMCPRosterHonoursAnExplicitTransport(t *testing.T) {
	path := writeRoster(t, `{"mcpServers": {
	  "legacy": {"baseUrl": "https://legacy.example/sse", "transport": "sse"}
	}}`)

	servers, err := LoadMCPRoster(path)
	if err != nil {
		t.Fatalf("LoadMCPRoster: %v", err)
	}
	// The shared shape carries no discriminator, so this is the one key Echo
	// adds: streamable and sse are both HTTP endpoints.
	if servers[0].ResolvedTransport() != MCPTransportSSE {
		t.Fatalf("transport = %q", servers[0].ResolvedTransport())
	}
}

func TestLoadMCPRosterRejectsAnInvalidServerName(t *testing.T) {
	path := writeRoster(t, `{"mcpServers": {"Not Valid": {"baseUrl": "https://x.example/mcp"}}}`)

	if _, err := LoadMCPRoster(path); err == nil {
		t.Fatal("an invalid server name was accepted")
	}
}
