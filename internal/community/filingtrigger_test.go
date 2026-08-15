package community

import (
	"path/filepath"
	"strings"
	"testing"
)

// Kai's filing decision on sirens-echo#235: any unanswerable in-scope question
// files, out-of-scope never does, and Echo and Deep get the same settings.

// The trigger was two named cases and is now any in-remit question the turn
// could not answer. A rule that lost the widening reads as the old one.
func TestTheFilingTriggerCoversAnyUnanswerableInScopeQuestion(t *testing.T) {
	t.Parallel()
	policy := issuePolicy("forgejo")
	for _, phrase := range []string{
		"inside this service's remit",
		"cannot be answered",
		// The rejected repeated-gap-only option, refused in the prose so a model
		// does not wait for a second occurrence before filing.
		"first time a gap",
	} {
		if !strings.Contains(policy, phrase) {
			t.Errorf("the filing trigger no longer says %q:\n%s", phrase, policy)
		}
	}
}

// Kai took the most responsive option knowing it files more, which makes the
// two limits on it load-bearing rather than decoration.
func TestTheWidenedTriggerKeepsItsTwoLimits(t *testing.T) {
	t.Parallel()
	policy := issuePolicy("forgejo")
	// One issue per turn. A turn that fails three lookups is one gap, not three.
	if !strings.Contains(policy, "one issue per turn") {
		t.Errorf("the per-turn limit is gone, so a turn can file once per failed lookup:\n%s", policy)
	}
	// The trigger is "in remit and unanswerable", never "unanswerable".
	if !strings.Contains(policy, "outside the remit") {
		t.Errorf("nothing excludes out-of-scope questions from filing:\n%s", policy)
	}
	// Duplicates are the accepted cost of the choice, so dedupe stops being a
	// nicety and becomes the thing keeping the tracker usable.
	if !strings.Contains(policy, "Search first") {
		t.Errorf("search-before-file is gone from a rule that now files more:\n%s", policy)
	}
}

// "Echo and Deep should have the same settings here" - Kai, sirens-echo#235.
// One function serves both, so this pins the property rather than the wiring.
func TestBothProfilesShareOneFilingPolicy(t *testing.T) {
	t.Parallel()
	definitions := map[string]string{
		"echo": filepath.Join("..", "..", "agent", "sirens-echo.yaml"),
		"deep": filepath.Join("..", "..", "agent", "sirens-deep.yaml"),
	}
	rendered := make(map[string]string, len(definitions))
	for name, path := range definitions {
		definition, err := LoadDefinition(path)
		if err != nil {
			t.Fatalf("load %s: %v", name, err)
		}
		if definition.IssueTracker == "" {
			t.Fatalf("%s names no issue tracker, so it renders no filing rule", name)
		}
		rendered[name] = issuePolicy(definition.IssueTracker)
	}
	if rendered["echo"] != rendered["deep"] {
		t.Errorf("the profiles carry different filing rules:\n%s\n---\n%s",
			rendered["echo"], rendered["deep"])
	}
}

// The gap is the subject of the issue. A member's words in a title would put
// Discord content in a public tracker, which no filing rate makes acceptable.
func TestTheFiledIssueNamesTheGapAndNotTheMember(t *testing.T) {
	t.Parallel()
	policy := issuePolicy("forgejo")
	if !strings.Contains(policy, "names the gap, never the member") {
		t.Errorf("nothing says the issue is about the gap rather than the member:\n%s", policy)
	}
	for _, phrase := range []string{"Never copy names", "personal details"} {
		if !strings.Contains(policy, phrase) {
			t.Errorf("the filing rule lost %q:\n%s", phrase, policy)
		}
	}
}

// A lane with no tracker cannot file, so it must not be told to. The widened
// trigger reaches only the branch that has somewhere to put an issue.
func TestALaneWithNoTrackerIsNotToldToFile(t *testing.T) {
	t.Parallel()
	policy := issuePolicy("")
	for _, phrase := range []string{"File when", "issue-tracker tool", "remit"} {
		if strings.Contains(policy, phrase) {
			t.Errorf("a trackerless lane is told to file: %q in\n%s", phrase, policy)
		}
	}
}
