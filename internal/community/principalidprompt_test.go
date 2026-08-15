package community

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// sirens-echo#310 saw Deep print the configured principal user ID to an
// unrecognised requester. 39de9fa took it out of the prompt; nothing held it out.

// The handle is checked alongside, so this cannot pass by the principal being
// empty. A vacuous guard here reads exactly like a real one.
func TestTheSystemPromptWithholdsThePrincipalUserID(t *testing.T) {
	t.Parallel()
	for _, name := range []string{"agents/echo/definition.yaml", "agents/deep/definition.yaml"} {
		definition, err := LoadDefinition(filepath.Join("..", "..", name))
		if err != nil {
			t.Fatalf("load %s: %v", name, err)
		}
		prompt := BuildSystemPrompt(definition, PlaceholderPrincipal, "", "policy")

		if strings.Contains(prompt, PlaceholderPrincipal.UserID) {
			t.Errorf("%s hands the model the principal user ID. It cannot decline "+
				"to disclose what it was never given. See sirens-echo#310", name)
		}
		if !strings.Contains(prompt, PlaceholderPrincipal.Handle) {
			t.Errorf("%s no longer carries the principal handle, so the check "+
				"above proves nothing about the identifier it does carry", name)
		}
	}
}

// The rendered snapshots are what ward exec prompt-check scores, so a template
// change that reintroduces the identifier has to fail here too.
func TestTheRenderedPromptsWithholdThePrincipalUserID(t *testing.T) {
	t.Parallel()
	for _, name := range []string{"agents/echo/rendered/prompt.txt", "agents/deep/rendered/prompt.txt"} {
		raw, err := os.ReadFile(filepath.Join("..", "..", name))
		if err != nil {
			t.Fatalf("read %s: %v", name, err)
		}
		body := string(raw)
		if strings.Contains(body, PlaceholderPrincipal.UserID) {
			t.Errorf("agent/rendered/%s carries the principal user ID", name)
		}
		if !strings.Contains(body, PlaceholderPrincipal.Handle) {
			t.Errorf("agent/rendered/%s carries no handle, so its check is vacuous", name)
		}
	}
}
