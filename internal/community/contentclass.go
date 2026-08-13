package community

import (
	"fmt"
	"os"
	"strings"

	"gopkg.in/yaml.v3"
)

// ContentClassSchema names the closed taxonomy the classifier chooses from.
const ContentClassSchema = "sirens-discord-ops.content-classes.v1"

// ContentClassOther is the explicit catch-all. Without it the set is not
// closed, and an unlisted request gets forced into a category it is not.
const ContentClassOther = "other"

// ContentClass is one class a request can be. Allowed classes are enumerated
// alongside denied ones so the model always has somewhere correct to land.
type ContentClass struct {
	ID      string `yaml:"id"`
	Summary string `yaml:"summary"`
	// Deny refuses the request. Absent means allowed, which is the default
	// boundary model rather than an omission.
	Deny bool `yaml:"deny"`
	// Sensitive changes the refusal shape and not the verdict. A sensitive
	// block names no category, because naming it says what to avoid next time.
	Sensitive bool `yaml:"sensitive"`
}

// ContentTaxonomy is the source-controlled class list.
type ContentTaxonomy struct {
	Schema  string         `yaml:"schema"`
	Classes []ContentClass `yaml:"classes"`
}

// LoadContentTaxonomy reads and validates the class list.
func LoadContentTaxonomy(path string) (ContentTaxonomy, error) {
	raw, err := os.ReadFile(path)
	if err != nil {
		return ContentTaxonomy{}, fmt.Errorf("read content taxonomy: %w", err)
	}
	var taxonomy ContentTaxonomy
	if err := yaml.Unmarshal(raw, &taxonomy); err != nil {
		return ContentTaxonomy{}, fmt.Errorf("parse content taxonomy: %w", err)
	}
	if taxonomy.Schema != ContentClassSchema {
		return ContentTaxonomy{}, fmt.Errorf(
			"unsupported content taxonomy schema %q", taxonomy.Schema,
		)
	}
	return taxonomy, taxonomy.validate()
}

// validate refuses a taxonomy that cannot do its job. See
// docs/sirens-echo-content-classes.md for why a set without a catch-all fails.
func (t ContentTaxonomy) validate() error {
	if len(t.Classes) == 0 {
		return fmt.Errorf("content taxonomy declares no classes")
	}
	seen := make(map[string]struct{}, len(t.Classes))
	allowed := 0
	catchAll := false
	for _, class := range t.Classes {
		if strings.TrimSpace(class.ID) == "" || strings.TrimSpace(class.Summary) == "" {
			return fmt.Errorf("content class requires id and summary")
		}
		if _, duplicate := seen[class.ID]; duplicate {
			return fmt.Errorf("content taxonomy declares %s twice", class.ID)
		}
		seen[class.ID] = struct{}{}
		if class.Sensitive && !class.Deny {
			return fmt.Errorf(
				"class %s is sensitive but not denied, which has no meaning",
				class.ID,
			)
		}
		if class.ID == ContentClassOther {
			if class.Deny {
				return fmt.Errorf("the catch-all class %s cannot deny", ContentClassOther)
			}
			catchAll = true
		}
		if !class.Deny {
			allowed++
		}
	}
	if !catchAll {
		return fmt.Errorf(
			"content taxonomy has no %s class, so it is not closed", ContentClassOther,
		)
	}
	if allowed < 2 {
		return fmt.Errorf("content taxonomy enumerates no allowed class besides the catch-all")
	}
	return nil
}

// Lookup returns a class by id. An unknown id means the classifier answered
// outside its own list, which is a failure rather than a default.
func (t ContentTaxonomy) Lookup(id string) (ContentClass, bool) {
	for _, class := range t.Classes {
		if strings.EqualFold(strings.TrimSpace(id), class.ID) {
			return class, true
		}
	}
	return ContentClass{}, false
}

// Verdict resolves matched classes. Sensitive wins ties, so a request tripping
// both never names the ordinary one. See docs/sirens-echo-content-classes.md.
func (t ContentTaxonomy) Verdict(ids []string) (ContentClass, bool, error) {
	decided := ContentClass{}
	blocked := false
	for _, id := range ids {
		class, known := t.Lookup(id)
		if !known {
			return ContentClass{}, false, fmt.Errorf("classifier returned unknown class %q", id)
		}
		if !class.Deny {
			continue
		}
		if !blocked || (class.Sensitive && !decided.Sensitive) {
			decided = class
		}
		blocked = true
	}
	return decided, blocked, nil
}

// ContentClassifierPrompt renders the instruction the classifying turn reads.
// A rule inside the reply prompt is an argument a member can win by repeating.
func ContentClassifierPrompt(taxonomy ContentTaxonomy) string {
	var out strings.Builder
	out.WriteString(
		"Classify the member's request into one or more of the classes below.\n" +
			"Answer with class ids only, comma separated, and nothing else.\n" +
			"Every request has a class. Use `other` when none of the rest fit.\n" +
			"Judge what the member asked for, not what a reply would say.\n\n",
	)
	for _, class := range taxonomy.Classes {
		fmt.Fprintf(&out, "- %s: %s\n", class.ID, strings.TrimSpace(class.Summary))
	}
	return out.String()
}
