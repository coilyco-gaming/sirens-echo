package community

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// A fixture rather than the deployed file, so this stays hermetic on a machine
// with no deploy checkout. See #122.
const guardfileFixture = `wrap ward mcp sirens-echo-forgejo {
    base-url "https://forgejo.coilysiren.me/api/v1"

    can get issue {
        path "/repos/coilyco-gaming/sirens-echo/issues/{index}"
    }
    can create issue {
        path "/repos/coilyco-gaming/sirens-echo/issues"
        body {
            field "title" type="string" required=true
        }
    }
    can add issue-label {
        path "/repos/coilyco-gaming/sirens-echo/issues/{index}/labels"
    }
}
`

func writeGuardfile(t *testing.T, body string) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "forgejo-mcp.mcp.kdl")
	if err := os.WriteFile(path, []byte(body), 0o644); err != nil {
		t.Fatalf("write fixture: %v", err)
	}
	return path
}

func TestGuardfileParsesItsGrants(t *testing.T) {
	t.Parallel()
	guard, err := ParseGuardfile(writeGuardfile(t, guardfileFixture))
	if err != nil {
		t.Fatalf("ParseGuardfile: %v", err)
	}
	if guard.Server != "sirens-echo-forgejo" {
		t.Errorf("server = %q", guard.Server)
	}
	if guard.Repository != "coilyco-gaming/sirens-echo" {
		t.Errorf("repository = %q", guard.Repository)
	}
	if len(guard.Grants) != 3 {
		t.Fatalf("grants = %#v", guard.Grants)
	}
	// Sorted, so the rendered skill does not churn on declaration order.
	want := []string{"add_issue-label", "create_issue", "get_issue"}
	for index, grant := range guard.Grants {
		if grant.Name() != want[index] {
			t.Errorf("grant %d = %s, want %s", index, grant.Name(), want[index])
		}
	}
}

// The skill has to state deny-by-absence, which is the half invisible from a
// tool list and the reason the skill exists at all.
func TestRenderedSkillStatesDenyByAbsence(t *testing.T) {
	t.Parallel()
	guard, err := ParseGuardfile(writeGuardfile(t, guardfileFixture))
	if err != nil {
		t.Fatalf("ParseGuardfile: %v", err)
	}
	rendered := RenderGuardfileSkill(guard)
	for _, expected := range []string{
		"denied by\nabsence",
		"exhaustive",
		"coilyco-gaming/sirens-echo",
		"no owner or repository argument",
		"`create_issue`",
		"edit an issue body",
	} {
		if !strings.Contains(rendered, expected) {
			t.Errorf("rendered skill missing %q", expected)
		}
	}
	// It must not claim a grant the guardfile does not carry.
	for _, absent := range []string{"`delete_issue`", "`edit_issue`", "`merge_pull-request`"} {
		if strings.Contains(rendered, absent) {
			t.Errorf("rendered skill claims %q", absent)
		}
	}
}

// A guardfile edit has to move the output, or the skill goes quietly stale.
func TestAGuardfileEditMovesTheRenderedSkill(t *testing.T) {
	t.Parallel()
	before, err := ParseGuardfile(writeGuardfile(t, guardfileFixture))
	if err != nil {
		t.Fatalf("ParseGuardfile: %v", err)
	}
	widened := strings.Replace(guardfileFixture,
		"    can add issue-label {",
		"    can close issue {\n        path \"/repos/coilyco-gaming/sirens-echo/issues/{index}\"\n    }\n    can add issue-label {",
		1)
	after, err := ParseGuardfile(writeGuardfile(t, widened))
	if err != nil {
		t.Fatalf("ParseGuardfile widened: %v", err)
	}
	if before.Digest == after.Digest {
		t.Error("a guardfile edit did not move the digest")
	}
	if RenderGuardfileSkill(before) == RenderGuardfileSkill(after) {
		t.Error("a new grant did not move the rendered skill")
	}
	if !strings.Contains(RenderGuardfileSkill(after), "`close_issue`") {
		t.Error("the new grant is missing from the rendered skill")
	}
}

func TestGuardfileWithNoGrantIsRefused(t *testing.T) {
	t.Parallel()
	if _, err := ParseGuardfile(writeGuardfile(t, "wrap ward mcp empty {\n}\n")); err == nil {
		t.Error("a guardfile declaring no grant was accepted")
	}
	if _, err := ParseGuardfile(filepath.Join(t.TempDir(), "absent.kdl")); err == nil {
		t.Error("a missing guardfile was accepted")
	}
}

// The tracked skill has to be what the generator produces, or the agent is
// describing a boundary nobody regenerated.
func TestTrackedGuardfileSkillLooksGenerated(t *testing.T) {
	t.Parallel()
	path := filepath.Join(
		"..", "..", ".agents", "skills", "coilyco-general", "references", "guardfile.md",
	)
	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read tracked skill: %v", err)
	}
	body := string(raw)
	for _, expected := range []string{
		"Generated from the deployed guardfile",
		"Do not edit by hand",
		"Source digest:",
		"denied by\nabsence",
	} {
		if !strings.Contains(body, expected) {
			t.Errorf("tracked skill missing %q", expected)
		}
	}
}
