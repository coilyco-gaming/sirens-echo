package community

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// The paths the pack's index names and the paths read_skill serves are one set.
// They were two, so every composed reference the prompt advertised was refused.

// writeBundleWithReference is writeBundle plus a deferred reference, which the
// shared helper deliberately has none of.
func writeBundleWithReference(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	skill := filepath.Join(dir, "content", "skills", "aos-public", "coding-go")
	if err := os.MkdirAll(filepath.Join(skill, "references"), 0o755); err != nil {
		t.Fatalf("make bundle dirs: %v", err)
	}
	write := func(path, body string) {
		t.Helper()
		if err := os.WriteFile(path, []byte(body), 0o644); err != nil {
			t.Fatalf("write %s: %v", path, err)
		}
	}
	// The card carries the surface ValidateSystemPrompt anchors on, so NewAgent
	// accepts the bundle rather than refusing it for a missing anchor.
	write(filepath.Join(dir, "content", "instructions.md"), PlaceholderComposed)
	write(filepath.Join(skill, "COMPOSED.md"), "# Go\n\nUse urfave/cli.")
	write(filepath.Join(skill, "references", "cli.md"), "# CLI reference\n\nFlag shapes.")
	return dir
}

// indexedReferencePaths reads the paths out of the pack's own index, so the
// assertion is against what the model is told rather than a second list.
func indexedReferencePaths(pack string) []string {
	_, index, found := strings.Cut(pack, "## Readable references")
	if !found {
		return nil
	}
	paths := make([]string, 0)
	for _, line := range strings.Split(index, "\n") {
		trimmed := strings.TrimSpace(line)
		if !strings.HasPrefix(trimmed, "- `") {
			continue
		}
		if path, _, ok := strings.Cut(strings.TrimPrefix(trimmed, "- `"), "`"); ok {
			paths = append(paths, path)
		}
	}
	return paths
}

func TestEveryBundleReferenceTheIndexNamesIsServable(t *testing.T) {
	t.Parallel()
	dir := writeBundleWithReference(t)

	pack, err := LoadBundle(dir)
	if err != nil {
		t.Fatalf("LoadBundle: %v", err)
	}
	indexed := indexedReferencePaths(pack)
	if len(indexed) == 0 {
		t.Fatal("the pack indexed no reference, so this test proves nothing")
	}

	references, err := LoadBundleReferences(dir)
	if err != nil {
		t.Fatalf("LoadBundleReferences: %v", err)
	}
	session, err := (&SkillProvider{References: references}).Open(context.Background())
	if err != nil {
		t.Fatalf("open skill session: %v", err)
	}
	defer session.Close()

	for _, path := range indexed {
		result, err := session.Call(
			context.Background(), skillToolName, map[string]any{"path": path})
		if err != nil {
			t.Fatalf("read_skill %s: %v", path, err)
		}
		if result.IsError {
			t.Errorf("the pack indexes %s and read_skill refuses it: %s", path, result.Text)
		}
	}
}

// A reference the pack inlined must not also be fetchable, or the same text is
// paid for twice. The partition is one walk, so this pins that it stays one.
func TestABundleReferenceIsEitherInlineOrFetchableNotBoth(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	skill := filepath.Join(dir, "content", "skills", "aos-public", "role-engineer")
	if err := os.MkdirAll(filepath.Join(skill, "references"), 0o755); err != nil {
		t.Fatalf("make bundle dirs: %v", err)
	}
	for path, body := range map[string]string{
		filepath.Join(dir, "content", "instructions.md"):    "# Role instructions\n\nCard.",
		filepath.Join(skill, "COMPOSED.md"):                 "# Engineer\n\nBuild it.",
		filepath.Join(skill, "references", "boundaries.md"): "---\ninline: always\n---\n\n# Boundaries\n\nDefer live changes.",
	} {
		if err := os.WriteFile(path, []byte(body), 0o644); err != nil {
			t.Fatalf("write %s: %v", path, err)
		}
	}
	pack, err := LoadBundle(dir)
	if err != nil {
		t.Fatalf("LoadBundle: %v", err)
	}
	if !strings.Contains(pack, "Defer live changes.") {
		t.Error("an inline: always reference did not reach the pack")
	}
	references, err := LoadBundleReferences(dir)
	if err != nil {
		t.Fatalf("LoadBundleReferences: %v", err)
	}
	for _, reference := range references {
		if strings.HasSuffix(reference.Path, "boundaries.md") {
			t.Error("an inlined reference is also fetchable, so its text is paid twice")
		}
	}
}

// The wiring, not just the partition. NewAgent built the skill tool from the
// definition's local roots alone, so a composed path was advertised and refused.
func TestTheAgentServesTheBundleReferencesItsPromptAdvertises(t *testing.T) {
	t.Parallel()
	agent, err := NewAgent(Config{
		Definition: Definition{
			Identity:      "Sirens Dowel of Coilyco",
			AuditRole:     "general",
			ResponseStyle: ResponseStyleSocial,
			Composed:      true,
			LocalSkillRoots: []string{
				filepath.Join("..", "..", ".agents", "skills", "coilyco-general"),
			},
		},
		BundlePath:      writeBundleWithReference(t),
		InstanceName:    "sirens-dowel",
		DiscordEnabled:  false,
		AgentProxyURL:   "http://agent-proxy:8080",
		AgentProxyModel: "sirens-echo/deepseek",
		HTTPListenAddr:  "127.0.0.1:0",
		RequestTimeout:  time.Second,
	}, nil)
	if err != nil {
		t.Fatalf("NewAgent: %v", err)
	}

	bundled := ""
	for _, path := range indexedReferencePaths(agent.systemPrompt) {
		if strings.Contains(path, "coding-go") {
			bundled = path
		}
	}
	if bundled == "" {
		t.Fatal("the prompt advertised no bundle reference, so this proves nothing")
	}

	proxy, ok := agent.completions.(ProxyClient)
	if !ok {
		t.Fatalf("completions is %T, so the tool provider cannot be reached", agent.completions)
	}
	session, err := proxy.Tools.Open(context.Background())
	if err != nil {
		t.Fatalf("open tool session: %v", err)
	}
	defer session.Close()

	result, err := session.Call(
		context.Background(), skillToolName, map[string]any{"path": bundled})
	if err != nil {
		t.Fatalf("read_skill %s: %v", bundled, err)
	}
	if result.IsError {
		t.Errorf("the prompt advertises %s and the agent's tool refuses it: %s",
			bundled, result.Text)
	}
}
