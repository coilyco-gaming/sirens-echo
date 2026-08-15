package community

import (
	"os"
	"path/filepath"
	"testing"

	"gopkg.in/yaml.v3"
)

// A declared short form is the same claim shortened, so it is refused the way
// the full identity is. See sirens-echo#559.
func TestADeclaredAliasIsRefusedLikeTheIdentity(t *testing.T) {
	t.Parallel()
	for _, reply := range []string{
		"Echo has filed a correction.",
		"Echo filed a correction.",
		"Echo has created a tracking issue.",
	} {
		if ValidateSelfAttributedClaim(reply, "Sirens Echo", []string{"Echo"}) == nil {
			t.Errorf("%q was allowed, though it is the identity shortened", reply)
		}
	}
}

// The organisation is the last word of one profile's identity, so a short form
// is declared rather than derived. sirens-echo#557's rule, restated.
func TestAnUndeclaredShortFormIsNotDerived(t *testing.T) {
	t.Parallel()
	for _, reply := range []string{
		"Coilyco filed a correction.",
		"Deep filed a correction.",
	} {
		if ValidateSelfAttributedClaim(reply, "Sirens Deep of Coilyco", nil) != nil {
			t.Errorf("%q was refused, though the profile declares no alias", reply)
		}
	}
}

// An alias is a word, not a substring, or every reply about echoes is refused.
func TestAnAliasDoesNotMatchInsideALongerWord(t *testing.T) {
	t.Parallel()
	for _, reply := range []string{
		"Echoes of the past were discussed.",
		"Echoing the earlier reply, the server is up.",
	} {
		if ValidateSelfAttributedClaim(reply, "Sirens Echo", []string{"Echo"}) != nil {
			t.Errorf("%q was refused, though no claim was made", reply)
		}
	}
}

// A blank alias would alternate to an empty branch, which matches everywhere.
func TestABlankAliasIsNotAnAlias(t *testing.T) {
	t.Parallel()
	if ValidateSelfAttributedClaim(
		"Kai filed a correction.", "Sirens Echo", []string{"", "   "},
	) != nil {
		t.Error("a blank alias matched a member's claim, so it alternated to nothing")
	}
}

// The corpus scores against a copy of Echo's aliases, and a copy drifts.
func TestTheCorpusAliasesMatchEchosDefinition(t *testing.T) {
	t.Parallel()
	raw, err := os.ReadFile(filepath.Join("..", "..", "agents", "echo", "definition.yaml"))
	if err != nil {
		t.Fatalf("read Echo's definition: %v", err)
	}
	var definition Definition
	if err := yaml.Unmarshal(raw, &definition); err != nil {
		t.Fatalf("parse Echo's definition: %v", err)
	}
	if len(definition.SelfAliases) != len(corpusSelfAliases) {
		t.Fatalf("definition declares %v, corpus scores %v",
			definition.SelfAliases, corpusSelfAliases)
	}
	for i, alias := range definition.SelfAliases {
		if alias != corpusSelfAliases[i] {
			t.Errorf("alias %d: definition %q, corpus %q", i, alias, corpusSelfAliases[i])
		}
	}
}
