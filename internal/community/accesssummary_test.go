package community

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// The summary the offline gate prints, so a reviewer can diff intent against
// effect rather than reading snowflakes. See sirens-echo#628.

// policyFile writes a policy and loads it the way the runtime does.
func policyFile(t *testing.T, body string) (*AccessPolicy, error) {
	t.Helper()
	path := filepath.Join(t.TempDir(), "access-policy.yaml")
	if err := os.WriteFile(path, []byte(body), 0o600); err != nil {
		t.Fatalf("write policy: %v", err)
	}
	return LoadAccessPolicy(path)
}

const boundedGuildPolicy = `schema: coilyco-harness.access.v1
guilds:
  - id: "700000000000000001"
    note: Sirens Echo
    channels: ["700000000000000002"]
    users: ["700000000000000003"]
`

// A policy the runtime accepts must produce a summary rather than an error, or
// the gate refuses every rollout including the correct ones.
func TestAGoodPolicyPassesAndIsDescribed(t *testing.T) {
	t.Parallel()
	policy, err := policyFile(t, boundedGuildPolicy)
	if err != nil {
		t.Fatalf("a valid policy was refused: %v", err)
	}
	summary := RenderAccessSummary(policy)
	for _, want := range []string{
		"guild 700000000000000001 (Sirens Echo)",
		"channels     1",
		"members      1",
		"per user     " + inheritedTier,
	} {
		if !strings.Contains(summary, want) {
			t.Errorf("summary is missing %q:\n%s", want, summary)
		}
	}
}

// The check this gate exists for. An open guild with no per-user bound is the
// one shape that turns an open guild into an unbounded one.
func TestAnOpenGuildWithoutAPerUserBoundIsRefused(t *testing.T) {
	t.Parallel()
	_, err := policyFile(t, `schema: coilyco-harness.access.v1
guilds:
  - id: "700000000000000001"
    channels: all
    users: all
`)
	if err == nil {
		t.Fatal("an unbounded open guild was accepted, which is what this gate is for")
	}
	if !strings.Contains(err.Error(), "rate_limit.per_user") {
		t.Errorf("the reason does not name the missing bound: %v", err)
	}
}

// The same guild with a real bound passes, so the check is about the bound and
// not about openness. Without this the row above passes by rejecting `all`.
func TestAnOpenGuildWithAPerUserBoundPasses(t *testing.T) {
	t.Parallel()
	policy, err := policyFile(t, `schema: coilyco-harness.access.v1
guilds:
  - id: "700000000000000001"
    channels: all
    users: all
    rate_limit:
      per_user: 1/60s
`)
	if err != nil {
		t.Fatalf("a bounded open guild was refused: %v", err)
	}
	summary := RenderAccessSummary(policy)
	if !strings.Contains(summary, "every member of this guild is admitted") {
		t.Errorf("the summary does not state the guild is open:\n%s", summary)
	}
	if !strings.Contains(summary, "per user     1 per 1m0s") {
		t.Errorf("the summary does not state the resolved bound:\n%s", summary)
	}
}

// `off` removes limiting and an absent tier inherits one. A summary that read
// them alike would hide the difference it exists to show.
func TestAnExplicitlyDisabledTierDoesNotReadAsInherited(t *testing.T) {
	t.Parallel()
	policy, err := policyFile(t, `schema: coilyco-harness.access.v1
guilds:
  - id: "700000000000000001"
    channels: all
    users: ["700000000000000003"]
    rate_limit:
      per_context: off
`)
	if err != nil {
		t.Fatalf("load: %v", err)
	}
	summary := RenderAccessSummary(policy)
	if !strings.Contains(summary, "per context  unlimited") {
		t.Errorf("a disabled tier does not read as unlimited:\n%s", summary)
	}
	if !strings.Contains(summary, "per user     "+inheritedTier) {
		t.Errorf("an unset tier does not read as inherited:\n%s", summary)
	}
}

// A malformed file fails rather than yielding an empty policy that admits
// nothing and looks like a narrow one.
func TestAnUnknownFieldIsRefused(t *testing.T) {
	t.Parallel()
	_, err := policyFile(t, `schema: coilyco-harness.access.v1
guildz:
  - id: "700000000000000001"
`)
	if err == nil {
		t.Fatal("a typo in a top-level key was accepted")
	}
}

// The summary reports the resolved entries, so it cannot describe a policy the
// runtime would have refused.
func TestNoPolicyIsSaidSoRatherThanRenderedEmpty(t *testing.T) {
	t.Parallel()
	if got := RenderAccessSummary(nil); !strings.Contains(got, "no access policy") {
		t.Errorf("a nil policy rendered as %q", got)
	}
}
