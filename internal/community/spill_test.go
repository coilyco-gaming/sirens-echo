package community

import (
	"context"
	"strings"
	"testing"
)

func spillSession(t *testing.T, requester string) ToolSession {
	t.Helper()
	provider := &ScratchProvider{Root: t.TempDir()}
	session, err := provider.Open(WithRequester(context.Background(), requester))
	if err != nil {
		t.Fatalf("open scratchpad: %v", err)
	}
	return session
}

// A trimmed result keeps its remainder where a scratchpad is mounted.
func TestSpillToolResultSavesTheRemainder(t *testing.T) {
	t.Parallel()
	session := spillSession(t, "member-1")
	full := strings.Repeat("t", maxToolResultBytes*2)

	path := spillToolResult(context.Background(), session, "get_trades", 0, full)
	if path != "tool-output/get_trades-1.txt" {
		t.Fatalf("spill path = %q", path)
	}

	read, err := session.Call(context.Background(), "scratch_read", map[string]any{"path": path})
	if err != nil {
		t.Fatalf("scratch_read: %v", err)
	}
	if !strings.Contains(read.Text, strings.Repeat("t", 64)) {
		t.Fatal("saved result does not carry the original content")
	}
}

// Two calls to one tool must not overwrite each other.
func TestSpillToolResultNumbersEachSave(t *testing.T) {
	t.Parallel()
	session := spillSession(t, "member-1")
	first := spillToolResult(context.Background(), session, "get_trades", 0, "alpha")
	second := spillToolResult(context.Background(), session, "get_trades", 1, "beta")
	if first == second {
		t.Fatalf("both saves took the same path %q", first)
	}
	if second != "tool-output/get_trades-2.txt" {
		t.Fatalf("second path = %q", second)
	}
}

// A deployment mounting no scratchpad keeps today's plain truncation.
func TestSpillToolResultWithoutScratchpadIsInert(t *testing.T) {
	t.Parallel()
	provider := &ScratchProvider{}
	session, err := provider.Open(context.Background())
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	if path := spillToolResult(context.Background(), session, "get_trades", 0, "alpha"); path != "" {
		t.Fatalf("saved without a scratchpad: %q", path)
	}
	if path := spillToolResult(context.Background(), nil, "get_trades", 0, "alpha"); path != "" {
		t.Fatalf("saved with no session: %q", path)
	}
}

// A result over the scratchpad file limit is refused, and a refusal must fall
// back rather than fail the turn.
func TestSpillToolResultOverFileLimitFallsBack(t *testing.T) {
	t.Parallel()
	session := spillSession(t, "member-1")
	oversized := strings.Repeat("t", maxScratchFileBytes+1)
	if path := spillToolResult(context.Background(), session, "get_trades", 0, oversized); path != "" {
		t.Fatalf("saved an oversized result: %q", path)
	}
}

// A server-supplied tool name must not reach the filesystem as a path.
func TestSpillPathFlattensToolNames(t *testing.T) {
	t.Parallel()
	for _, tool := range []string{"../../etc/passwd", "a/b", "..", ""} {
		got := spillPath(tool, 0)
		if strings.Contains(got, "..") || strings.Count(got, "/") != 1 {
			t.Fatalf("spillPath(%q) = %q", tool, got)
		}
	}
}

// A traversal attempt is refused by the scratchpad itself, not only by naming.
func TestSpillToolResultRefusesTraversalName(t *testing.T) {
	t.Parallel()
	session := spillSession(t, "member-1")
	path := spillToolResult(context.Background(), session, "../../escape", 0, "alpha")
	if strings.Contains(path, "..") {
		t.Fatalf("traversal survived: %q", path)
	}
}
