package community

import (
	"strings"
	"testing"
)

// A supplied roster path that resolves to nothing is a mistake, and it used to
// be indistinguishable from a deliberate no-tool boundary. See #684.

// The shape that motivated this: deploy stores the roster as a ConfigMap, and
// the inner document is what the pod mounts.
func TestAConfigMapIsRefusedRatherThanReadAsEmpty(t *testing.T) {
	t.Parallel()
	path := writeRoster(t, `apiVersion: v1
kind: ConfigMap
metadata:
  name: sirens-echo-mcp-roster
data:
  mcp-roster.yml: |
    mcpServers:
      eco:
        url: https://eco-app.coilysiren.me/mcp
`)
	servers, err := LoadMCPRoster(path)
	if err == nil {
		t.Fatalf("a ConfigMap loaded as %d servers instead of failing", len(servers))
	}
	if !strings.Contains(err.Error(), "ConfigMap") {
		t.Errorf("the error does not name the likely cause: %v", err)
	}
}

func TestARosterNamingNoServersIsRefused(t *testing.T) {
	t.Parallel()
	for name, body := range map[string]string{
		"empty file":    "",
		"empty mapping": "mcpServers: {}\n",
		"absent key":    "somethingElse: true\n",
		"comments only": "# nothing here\n",
	} {
		if _, err := LoadMCPRoster(writeRoster(t, body)); err == nil {
			t.Errorf("%s was accepted as a roster", name)
		}
	}
}

// The real shape still loads, or the guard traded a silent failure for a loud
// one on correct input.
func TestARealRosterStillLoads(t *testing.T) {
	t.Parallel()
	servers, err := LoadMCPRoster(writeRoster(t, `mcpServers:
  eco:
    url: https://eco-app.coilysiren.me/mcp
`))
	if err != nil {
		t.Fatalf("a valid roster was refused: %v", err)
	}
	if len(servers) != 1 || servers[0].Name != "eco" {
		t.Errorf("servers = %+v, want the one named", servers)
	}
}
