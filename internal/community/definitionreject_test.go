package community

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// LoadDefinition is the only thing standing between a malformed definition and
// a prompt built from it. Every branch here refuses; none of them was exercised.

// validDefinition is the shape agent/sirens-echo.yaml has. Each case below
// breaks exactly one field, so a rejection is attributable to that field.
const validDefinition = `schema: coilyco-harness.agent.v1
identity: Sirens Echo
audit_role: community
response_style: neutral
channel: "#bots"
max_context_messages: 12
local_skill_roots:
  - .agents/skills/sirens-echo-community
issue_tracker: forgejo
`

// writeDefinition puts a definition on disk and returns its path.
func writeDefinition(t *testing.T, body string) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "definition.yaml")
	if err := os.WriteFile(path, []byte(body), 0o600); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}
	return path
}

// The base has to load, or every rejection below passes for the wrong reason.
func TestTheDefinitionFixtureIsItselfValid(t *testing.T) {
	t.Parallel()
	if _, err := LoadDefinition(writeDefinition(t, validDefinition)); err != nil {
		t.Fatalf("the valid fixture was rejected, so the corpus below proves nothing: %v", err)
	}
}

// One broken field per row, and the message has to name the field. A rejection
// that does not say which field sends an operator to read the parser.
func TestLoadDefinitionRefusesEachMalformedField(t *testing.T) {
	t.Parallel()
	for _, row := range []struct {
		name    string
		old     string
		new     string
		mustSay string
	}{
		{"unsupported schema", "coilyco-harness.agent.v1", "coilyco-harness.agent.v2", "schema"},
		{"absent schema", "schema: coilyco-harness.agent.v1\n", "", "schema"},
		{"absent identity", "identity: Sirens Echo\n", "", "identity"},
		{"absent audit role", "audit_role: community\n", "", "audit_role"},
		{"channel without a hash", `channel: "#bots"`, `channel: "bots"`, "channel"},
		{"channel with a space", `channel: "#bots"`, `channel: "#two words"`, "channel"},
		{"unsupported response style", "response_style: neutral", "response_style: chatty", "response_style"},
		{"absent response style", "response_style: neutral\n", "", "response_style"},
		{"context window of zero", "max_context_messages: 12", "max_context_messages: 0", "max_context_messages"},
		{"context window past the ceiling", "max_context_messages: 12", "max_context_messages: 51", "max_context_messages"},
		{"negative context window", "max_context_messages: 12", "max_context_messages: -1", "max_context_messages"},
		{"no skill roots", "local_skill_roots:\n  - .agents/skills/sirens-echo-community\n", "", "skill root"},
		{"issue tracker with a capital", "issue_tracker: forgejo", "issue_tracker: Forgejo", "issue_tracker"},
		{"issue tracker with a slash", "issue_tracker: forgejo", "issue_tracker: forge/jo", "issue_tracker"},
	} {
		t.Run(row.name, func(t *testing.T) {
			t.Parallel()
			body := strings.Replace(validDefinition, row.old, row.new, 1)
			if body == validDefinition {
				t.Fatalf("the fixture does not contain %q, so this row changed nothing", row.old)
			}
			_, err := LoadDefinition(writeDefinition(t, body))
			if err == nil {
				t.Fatalf("accepted a definition with %s", row.name)
			}
			if !strings.Contains(err.Error(), row.mustSay) {
				t.Errorf("%v, expected the message to name %q", err, row.mustSay)
			}
		})
	}
}

// A file that is not YAML and a file that is not there are different failures,
// and an operator needs to know which one they have.
func TestLoadDefinitionSeparatesAMissingFileFromABrokenOne(t *testing.T) {
	t.Parallel()
	absent := filepath.Join(t.TempDir(), "nothing.yaml")
	_, err := LoadDefinition(absent)
	if err == nil {
		t.Fatal("a definition that does not exist was accepted")
	}
	if !strings.Contains(err.Error(), "read agent definition") {
		t.Errorf("%v, expected the message to say the file could not be read", err)
	}

	_, err = LoadDefinition(writeDefinition(t, "schema: [unclosed\n"))
	if err == nil {
		t.Fatal("a definition that is not YAML was accepted")
	}
	if !strings.Contains(err.Error(), "parse agent definition") {
		t.Errorf("%v, expected the message to say the file could not be parsed", err)
	}
}

// The channel label is optional, so an empty one has to stay accepted. This is
// the must-not-fire half: a tightened pattern that rejects it breaks deployment.
func TestLoadDefinitionAcceptsAnAbsentChannelLabel(t *testing.T) {
	t.Parallel()
	body := strings.Replace(validDefinition, "channel: \"#bots\"\n", "", 1)
	if _, err := LoadDefinition(writeDefinition(t, body)); err != nil {
		t.Fatalf("a definition with no channel label was rejected: %v", err)
	}
	// Same for the tracker, which deployment may leave unset.
	body = strings.Replace(validDefinition, "issue_tracker: forgejo\n", "", 1)
	if _, err := LoadDefinition(writeDefinition(t, body)); err != nil {
		t.Fatalf("a definition with no issue tracker was rejected: %v", err)
	}
}

// The bounds are inclusive at both ends. An off-by-one here silently narrows
// what a deployment may configure.
func TestLoadDefinitionAcceptsBothEndsOfTheContextWindow(t *testing.T) {
	t.Parallel()
	for _, value := range []string{"1", "50"} {
		body := strings.Replace(validDefinition, "max_context_messages: 12",
			"max_context_messages: "+value, 1)
		if _, err := LoadDefinition(writeDefinition(t, body)); err != nil {
			t.Errorf("max_context_messages: %s was rejected: %v", value, err)
		}
	}
}
