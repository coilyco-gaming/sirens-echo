package community

import (
	"strings"
	"testing"
)

// A persona shapes voice. It must not be in a position to override policy, and
// the mechanism for that is order: policy is read after it. See sirens-echo#237.

func TestPolicyIsReadAfterTheComposedPersona(t *testing.T) {
	t.Parallel()
	prompt := BuildSystemPrompt(
		Definition{
			Identity:      "Sirens Deep of Coilyco",
			AuditRole:     "general",
			ResponseStyle: ResponseStyleSocial,
		},
		PlaceholderPrincipal,
		"a composed persona, whatever it happens to say",
		"the local policy root",
	)

	persona := strings.Index(prompt, "<composed-identity>")
	if persona < 0 {
		t.Fatal("the composed persona did not render, so this asserts nothing")
	}
	// Each of these carries a boundary a persona must not be able to relax.
	for _, after := range []struct{ name, marker string }{
		{"the local policy root", "<local-policy>"},
		{"the action-claim constraint", "Never claim that an issue"},
		{"the reply format", "Reply with plain text"},
	} {
		at := strings.Index(prompt, after.marker)
		if at < 0 {
			t.Errorf("%s is missing from the prompt entirely", after.name)
			continue
		}
		if at < persona {
			t.Errorf("%s is read before the composed persona, so a persona would "+
				"have the last word on it. Policy must follow the persona", after.name)
		}
	}
}

// The persona is caller-supplied content in a prompt whose whole design assumes
// caller content cannot widen anything, so it must not sit outside its tag.
func TestTheComposedPersonaStaysInsideItsTag(t *testing.T) {
	t.Parallel()
	prompt := BuildSystemPrompt(
		Definition{Identity: "Sirens Deep of Coilyco", AuditRole: "general", ResponseStyle: ResponseStyleSocial},
		PlaceholderPrincipal,
		"PERSONA-SENTINEL",
		"the local policy root",
	)
	open := strings.Index(prompt, "<composed-identity>")
	closed := strings.Index(prompt, "</composed-identity>")
	at := strings.Index(prompt, "PERSONA-SENTINEL")
	if open < 0 || closed < 0 || at < 0 {
		t.Fatal("the composed persona or its tag did not render")
	}
	if at < open || at > closed {
		t.Error("the persona rendered outside its tag, so a reader cannot tell " +
			"where caller-supplied identity ends and harness policy begins")
	}
}
