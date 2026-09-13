package community

import (
	"strings"
	"testing"
)

// The name-only deny tests build their fixture from the deny list, so they hold
// whatever the catalogue is really called. These cover that gap.

func TestDenyListResolvesAgainstACatalogueHoldingEveryLiveEntry(t *testing.T) {
	t.Parallel()
	names := make([]string, 0, len(DeniedComposedSkills))
	for name := range DeniedComposedSkills {
		names = append(names, name)
	}
	catalog := fixtureCatalog(t, names...)
	if err := VerifyDenyListResolves([]string{catalog}); err != nil {
		t.Fatalf("a catalogue holding every live entry was rejected: %v", err)
	}
}

// The failure this exists for: an entry naming a source that was renamed.
func TestDenyListRefusesAnEntryThatResolvesNowhere(t *testing.T) {
	t.Parallel()
	names := make([]string, 0, len(DeniedComposedSkills))
	for name := range DeniedComposedSkills {
		names = append(names, name)
	}
	// Drop one live entry, standing in for a source renamed underneath it.
	renamed := names[0]
	catalog := fixtureCatalog(t, names[1:]...)
	err := VerifyDenyListResolves([]string{catalog})
	if err == nil {
		t.Fatal("a deny entry resolving nowhere was accepted")
	}
	if !strings.Contains(err.Error(), renamed) {
		t.Errorf("%v, expected the error to name %q", err, renamed)
	}
	if !strings.Contains(err.Error(), "guards nothing") {
		t.Errorf("%v, expected the error to say the entry guards nothing", err)
	}
}

// A retired name coming back must be re-verified rather than trusted, since it
// is the half of the list nothing else checks.
func TestDenyListRefusesARetiredEntryThatCameBack(t *testing.T) {
	t.Parallel()
	if len(RetiredDeniedSkills) == 0 {
		t.Skip("no retired entries to check")
	}
	names := make([]string, 0, len(DeniedComposedSkills))
	for name := range DeniedComposedSkills {
		names = append(names, name)
	}
	revived := ""
	for name := range RetiredDeniedSkills {
		revived = name
		break
	}
	catalog := fixtureCatalog(t, append(names, revived)...)
	err := VerifyDenyListResolves([]string{catalog})
	if err == nil {
		t.Fatal("a retired entry back in a catalogue was accepted")
	}
	if !strings.Contains(err.Error(), revived) {
		t.Errorf("%v, expected the error to name %q", err, revived)
	}
}

// Retiring a name must not stop it being refused, or retirement becomes a way
// to quietly re-admit private context.
func TestRetiredSkillsAreStillDenied(t *testing.T) {
	t.Parallel()
	for retired, reason := range RetiredDeniedSkills {
		catalog := fixtureCatalog(t, retired, "writing-house-style")
		graph := RoleGraph{Patterns: map[string][]string{"creator": {retired}}}
		admitted, _, err := ExpandRoleWithExclusions([]string{catalog}, "creator", graph)
		if err == nil {
			t.Errorf("%q was accepted when named exactly", retired)
			continue
		}
		if _, leaked := admitted[retired]; leaked {
			t.Errorf("%q was admitted alongside its own error", retired)
		}
		if !strings.Contains(err.Error(), reason) {
			t.Errorf("%q: %v, expected the reason %q", retired, err, reason)
		}
	}
}

// The two halves must stay disjoint, or one name's verdict depends on map order.
func TestDenyListHalvesAreDisjoint(t *testing.T) {
	t.Parallel()
	for name := range RetiredDeniedSkills {
		if _, both := DeniedComposedSkills[name]; both {
			t.Errorf("%q is both live and retired", name)
		}
	}
}
