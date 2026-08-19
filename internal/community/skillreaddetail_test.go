package community

import (
	"context"
	"strings"
	"testing"
)

// A resolved skill read names the reference it delivered, and only a
// session-validated name ever reaches a member surface. See docs/sirens-echo-worklog.md.

func skillDetailSession(t *testing.T) ToolSession {
	t.Helper()
	provider := &SkillProvider{References: []SkillReference{{
		Path:  "references/astronomy.md",
		Title: "Astronomy",
		Body:  "the stars",
	}}}
	session, err := provider.Open(context.Background())
	if err != nil {
		t.Fatalf("open skill provider: %v", err)
	}
	return session
}

func TestSkillReadCarriesValidatedDetail(t *testing.T) {
	session := skillDetailSession(t)
	result, err := session.Call(context.Background(), skillToolName,
		map[string]any{"path": "references/astronomy.md"})
	if err != nil {
		t.Fatalf("read: %v", err)
	}
	if result.Detail != "astronomy" {
		t.Fatalf("detail = %q, want the file's own name", result.Detail)
	}
}

func TestSkillRefusalCarriesNoDetail(t *testing.T) {
	session := skillDetailSession(t)
	result, err := session.Call(context.Background(), skillToolName,
		map[string]any{"path": "references/guessed.md"})
	if err != nil {
		t.Fatalf("refusal is a result, not an error: %v", err)
	}
	if result.Detail != "" {
		t.Fatalf("a refusal must not echo a name, got %q", result.Detail)
	}
}

func TestWorklogRowNamesResolvedSkillRead(t *testing.T) {
	row := progressRow{
		server: skillToolServer, tool: skillToolName,
		outcome: ToolOutcomeOK, done: true, detail: "astronomy",
	}
	rendered := worklogRow(row)
	want := "> " + skillReadGlyph + " " + toolOKGlyph + " `astronomy`"
	if rendered != want {
		t.Fatalf("row = %q, want %q", rendered, want)
	}
	if !noticeShape.MatchString(rendered) {
		t.Fatalf("row %q escapes the notice shape", rendered)
	}
}

func TestWorklogRowInFlightWearsTheBook(t *testing.T) {
	row := progressRow{server: skillToolServer, tool: skillToolName, detail: "astronomy"}
	rendered := worklogRow(row)
	if strings.Contains(rendered, "astronomy") {
		t.Fatalf("an unresolved row must not carry a detail, got %q", rendered)
	}
	if want := "> " + skillReadGlyph + " `skills.read_skill`"; rendered != want {
		t.Fatalf("in-flight skill row = %q, want %q", rendered, want)
	}
}

func TestDisclosureNamesSkillRead(t *testing.T) {
	footer := toolDisclosure([]ExecutedTool{{
		Name: skillToolName, Server: skillToolServer,
		Original: skillToolName, Outcome: ToolOutcomeOK, Detail: "astronomy",
	}})
	want := "> " + skillReadGlyph + " " + toolOKGlyph + " `astronomy`"
	if footer != want {
		t.Fatalf("footer = %q, want %q", footer, want)
	}
}

func TestDisclosureNeverCollapsesAcrossDetails(t *testing.T) {
	call := func(detail string) ExecutedTool {
		return ExecutedTool{
			Name: skillToolName, Server: skillToolServer,
			Original: skillToolName, Outcome: ToolOutcomeOK, Detail: detail,
		}
	}
	footer := toolDisclosure([]ExecutedTool{
		call("astronomy"), call("astronomy"), call("geology"),
	})
	lines := strings.Split(footer, "\n")
	if len(lines) != 2 {
		t.Fatalf("want the repeat collapsed and the other read kept, got %q", footer)
	}
	if !strings.Contains(lines[0], "×2") || !strings.Contains(lines[0], "astronomy") {
		t.Fatalf("same reference at same status should collapse, got %q", lines[0])
	}
	if !strings.Contains(lines[1], "geology") {
		t.Fatalf("a different reference must break the run, got %q", lines[1])
	}
}
