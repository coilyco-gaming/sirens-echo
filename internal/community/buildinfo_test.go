package community

import (
	"strings"
	"testing"
)

// These mutate the package-level build stamp, so they do not run in parallel
// with each other or with anything else reading it.

const sampleRevision = "70fa2743c1b84f4d0f3a2e7c9b5d1a6e8c4f0b27"

func withRevision(t *testing.T, revision string) {
	t.Helper()
	previous := buildRevision
	buildRevision = revision
	t.Cleanup(func() { buildRevision = previous })
}

func neutralDefinition() Definition {
	return Definition{
		Identity:      "Sirens Echo",
		AuditRole:     "community",
		ResponseStyle: ResponseStyleNeutral,
		Channel:       "#bots",
		IssueTracker:  "forgejo",
	}
}

// A stamped build is the only thing that can name the running commit, so the
// prompt carries it or the reference permits a link nobody can construct.
func TestSystemPromptNamesTheBuildRevisionWhenStamped(t *testing.T) {
	withRevision(t, sampleRevision)
	prompt := BuildSystemPrompt(neutralDefinition(), PlaceholderPrincipal, "", "policy")
	for _, expected := range []string{
		sampleRevision,
		"src/commit/" + sampleRevision + "/<path>",
		"names the code actually running",
	} {
		if !strings.Contains(prompt, expected) {
			t.Errorf("a stamped prompt is missing %q", expected)
		}
	}
	if err := ValidateNeutralSystemPrompt(false, prompt); err != nil {
		t.Errorf("the revision section broke the neutral validator: %v", err)
	}
}

// An unstamped build must say nothing rather than describe a revision it does
// not have, which is the whole reason the section is conditional.
func TestSystemPromptOmitsTheRevisionWhenUnstamped(t *testing.T) {
	withRevision(t, "")
	prompt := BuildSystemPrompt(neutralDefinition(), PlaceholderPrincipal, "", "policy")
	for _, forbidden := range []string{"This build is commit", "src/commit/"} {
		if strings.Contains(prompt, forbidden) {
			t.Errorf("an unstamped prompt still carries %q", forbidden)
		}
	}
	if BuildRevision() != "" {
		t.Errorf("BuildRevision reported %q for an unstamped build", BuildRevision())
	}
}

// The pinned form and the branch form are different paths, and a reply that
// mixes them names source that is not what answered.
func TestRevisionPolicyPinsACommitRatherThanABranch(t *testing.T) {
	withRevision(t, sampleRevision)
	section := revisionPolicy()
	if strings.Contains(section, "/src/branch/") {
		t.Error("the revision section offers a branch link, which is not the running build")
	}
	if !strings.Contains(section, "/src/commit/") {
		t.Error("the revision section does not offer a commit-pinned link")
	}
}
