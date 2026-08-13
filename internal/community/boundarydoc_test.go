package community

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// A denied class with no rule in a policy root is a boundary that exists on
// paper. Nothing reads the taxonomy yet, so the prose is the only enforcement.

// policyRootProse is every file the Echo model actually reads, reflowed, since
// a phrase that wraps across a line would otherwise skip the check silently.
func policyRootProse(t *testing.T) string {
	t.Helper()
	paths := []string{
		"../../.agents/skills/sirens-echo-knowledge/references/boundaries.md",
		"../../.agents/skills/sirens-echo-community/SKILL.md",
	}
	var joined strings.Builder
	for _, path := range paths {
		body, err := os.ReadFile(path)
		if err != nil {
			t.Fatalf("read %s: %v", filepath.Base(path), err)
		}
		joined.WriteString(strings.Join(strings.Fields(string(body)), " "))
		joined.WriteString(" ")
	}
	return joined.String()
}

// deniedClassRules maps a denied class to a phrase its rule must contain. A new
// class fails here until someone writes the rule and records the phrase.
var deniedClassRules = map[string]string{
	"nsfw":                 "Sexual or explicit content",
	"minor-suspected":      "states or implies they are a child",
	"creative-long-form":   "Extended original fiction",
	"medical-legal-advice": "Diagnosis, treatment, dosage",
	"irl-physical":         "Acting on the physical world",
	"emotional-support":    "Emotional support is an ordinary decline",
}

func TestEveryDeniedClassHasARuleTheModelReads(t *testing.T) {
	t.Parallel()
	taxonomy, err := LoadContentTaxonomy(filepath.Join("..", "..", "agent", "content-classes.yaml"))
	if err != nil {
		t.Fatalf("load taxonomy: %v", err)
	}
	prose := policyRootProse(t)

	denied := 0
	for _, class := range taxonomy.Classes {
		if !class.Deny {
			continue
		}
		denied++
		phrase, recorded := deniedClassRules[class.ID]
		if !recorded {
			t.Errorf("denied class %s has no rule recorded here. Write the rule "+
				"into the policy root and add its phrase to deniedClassRules, or "+
				"the class denies nothing the model can act on", class.ID)
			continue
		}
		if !strings.Contains(prose, phrase) {
			t.Errorf("denied class %s expects the phrase %q in a policy root and "+
				"it is not there", class.ID, phrase)
		}
	}
	if denied == 0 {
		t.Fatal("the taxonomy denies nothing, so this test is checking an empty set")
	}
}

// A sensitive refusal must not name its category, so the rule that carries it
// has to say so or the shape is left to the model to guess.
func TestSensitiveRefusalsAreToldNotToNameTheReason(t *testing.T) {
	t.Parallel()
	taxonomy, err := LoadContentTaxonomy(filepath.Join("..", "..", "agent", "content-classes.yaml"))
	if err != nil {
		t.Fatalf("load taxonomy: %v", err)
	}
	sensitive := 0
	for _, class := range taxonomy.Classes {
		if class.Sensitive {
			sensitive++
		}
	}
	if sensitive == 0 {
		t.Skip("no sensitive class to carry the shape rule")
	}
	prose := policyRootProse(t)
	for _, expected := range []string{
		"Decline without naming the reason",
		"Naming the rule tells the next person what to avoid saying",
	} {
		if !strings.Contains(prose, expected) {
			t.Errorf("%d sensitive classes exist but no policy root says %q",
				sensitive, expected)
		}
	}
}
