package community

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// testPartition is where a requester's own writes land under the test session,
// so a layout change moves one helper rather than every test.
func testPartition(root, requester string) string {
	return filepath.Join(
		root,
		DirectSession("test-channel", requester).Directory(),
		scratchPartitionName(requester),
	)
}

// scratchContext supplies both partitions a turn carries. Production derives
// the session from the turn; a test names one so the wiring is explicit.
func scratchContext(requester string) context.Context {
	return WithSession(
		WithRequester(context.Background(), requester),
		DirectSession("test-channel", requester),
	)
}

func openScratch(t *testing.T, root, requester string) ToolSession {
	t.Helper()
	provider := &ScratchProvider{Root: root}
	session, err := provider.Open(scratchContext(requester))
	if err != nil {
		t.Fatalf("open scratchpad: %v", err)
	}
	t.Cleanup(func() { _ = session.Close() })
	return session
}

func callScratch(
	t *testing.T,
	session ToolSession,
	name string,
	arguments map[string]any,
) ToolResult {
	t.Helper()
	result, err := session.Call(context.Background(), name, arguments)
	if err != nil {
		t.Fatalf("call %s: %v", name, err)
	}
	return result
}

// An unmounted scratchpad offers nothing, so the schemas never reach a prompt
// on a deployment that did not ask for the capability.
func TestScratchDisabledOffersNoTools(t *testing.T) {
	provider := &ScratchProvider{}
	session, err := provider.Open(context.Background())
	if err != nil {
		t.Fatalf("open disabled scratchpad: %v", err)
	}
	if tools := session.Tools(); len(tools) != 0 {
		t.Fatalf("disabled scratchpad offered %d tools", len(tools))
	}
	if _, err := session.Call(context.Background(), "scratch_read", nil); err == nil {
		t.Fatal("disabled scratchpad answered a call")
	}
}

// Absence of a principal is denial. A turn with nothing to attribute to gets no
// partition rather than a shared one.
func TestScratchRefusesUnattributedTurn(t *testing.T) {
	provider := &ScratchProvider{Root: t.TempDir()}
	if _, err := provider.Open(context.Background()); err == nil {
		t.Fatal("scratchpad opened without a requester")
	}
}

func TestScratchWriteThenReadRoundTrips(t *testing.T) {
	session := openScratch(t, t.TempDir(), "111")
	write := callScratch(t, session, "scratch_write", map[string]any{
		"path": "notes/plan.md", "content": "one\ntwo\n",
	})
	if write.IsError {
		t.Fatalf("write refused: %s", write.Text)
	}
	read := callScratch(t, session, "scratch_read", map[string]any{"path": "notes/plan.md"})
	if read.IsError || read.Text != "one\ntwo\n" {
		t.Fatalf("read returned %q (error=%v)", read.Text, read.IsError)
	}
	list := callScratch(t, session, "scratch_list", nil)
	if !strings.Contains(list.Text, "notes/plan.md") {
		t.Fatalf("listing missed the file: %s", list.Text)
	}
	found := callScratch(t, session, "scratch_search", map[string]any{"query": "TWO"})
	if !strings.Contains(found.Text, "plan.md:2") {
		t.Fatalf("search missed the line: %s", found.Text)
	}
}

// Two requesters on one volume must not see each other. This is the property
// the capability rests on. See docs/sirens-echo-scratchpad.md.
func TestScratchPartitionsPerRequester(t *testing.T) {
	root := t.TempDir()
	first := openScratch(t, root, "111")
	second := openScratch(t, root, "222")

	callScratch(t, first, "scratch_write", map[string]any{
		"path": "secret.txt", "content": "first only",
	})

	list := callScratch(t, second, "scratch_list", nil)
	if strings.Contains(list.Text, "secret.txt") {
		t.Fatalf("second requester saw the first's file: %s", list.Text)
	}
	read := callScratch(t, second, "scratch_read", map[string]any{"path": "secret.txt"})
	if !read.IsError {
		t.Fatalf("second requester read the first's file: %s", read.Text)
	}
	found := callScratch(t, second, "scratch_search", map[string]any{"query": "first only"})
	if strings.Contains(found.Text, "secret.txt") {
		t.Fatalf("second requester searched the first's file: %s", found.Text)
	}
}

// Traversal is refused on where the path lands, not on how it is spelled.
func TestScratchRefusesEscape(t *testing.T) {
	root := t.TempDir()
	if err := os.WriteFile(filepath.Join(root, "outside.txt"), []byte("no"), 0o600); err != nil {
		t.Fatalf("seed: %v", err)
	}
	session := openScratch(t, root, "111")

	for _, path := range []string{
		"../outside.txt",
		"../../etc/passwd",
		"notes/../../outside.txt",
		"/etc/passwd",
		"./../outside.txt",
	} {
		read := callScratch(t, session, "scratch_read", map[string]any{"path": path})
		if !read.IsError {
			t.Fatalf("read escaped with %q: %s", path, read.Text)
		}
		write := callScratch(t, session, "scratch_write", map[string]any{
			"path": path, "content": "owned",
		})
		if !write.IsError {
			t.Fatalf("write escaped with %q: %s", path, write.Text)
		}
	}
	if _, err := os.ReadFile(filepath.Join(root, "outside.txt")); err != nil {
		t.Fatalf("seed file disturbed: %v", err)
	}
}

// A symlink is followed before the decision, so it cannot be used as a door.
func TestScratchRefusesSymlinkEscape(t *testing.T) {
	root := t.TempDir()
	outside := t.TempDir()
	if err := os.WriteFile(filepath.Join(outside, "target.txt"), []byte("no"), 0o600); err != nil {
		t.Fatalf("seed: %v", err)
	}
	session := openScratch(t, root, "111")
	// Derived, not spelled. The partition is a hash of the requester, so a
	// literal name plants the door somewhere the session never looks.
	partition := testPartition(root, "111")
	if err := os.Symlink(outside, filepath.Join(partition, "door")); err != nil {
		// Only a platform that cannot make symlinks may skip. Any other error
		// means the door was never planted, so the escape below is not tried.
		if errors.Is(err, errors.ErrUnsupported) {
			t.Skipf("symlinks unsupported on this platform: %v", err)
		}
		t.Fatalf("plant a symlink in %s: %v", partition, err)
	}

	read := callScratch(t, session, "scratch_read", map[string]any{"path": "door/target.txt"})
	if !read.IsError {
		t.Fatalf("read followed a symlink out: %s", read.Text)
	}
	write := callScratch(t, session, "scratch_write", map[string]any{
		"path": "door/planted.txt", "content": "owned",
	})
	if !write.IsError {
		t.Fatalf("write followed a symlink out: %s", write.Text)
	}
	if _, err := os.Stat(filepath.Join(outside, "planted.txt")); !os.IsNotExist(err) {
		t.Fatal("a file was planted outside the partition")
	}
}

func TestScratchEnforcesFileCeiling(t *testing.T) {
	session := openScratch(t, t.TempDir(), "111")
	result := callScratch(t, session, "scratch_write", map[string]any{
		"path": "big.txt", "content": strings.Repeat("x", maxScratchFileBytes+1),
	})
	if !result.IsError {
		t.Fatal("an oversized file was accepted")
	}
}

// The per-requester ceiling binds across sessions, because inside one the
// smaller session cap evicts first. See docs/sirens-echo-session-workspace.md.
func TestScratchEnforcesRequesterCeilingAcrossSessions(t *testing.T) {
	root := t.TempDir()
	provider := &ScratchProvider{Root: root}
	chunk := strings.Repeat("y", maxScratchFileBytes)
	perSession := maxSessionBytes / maxScratchFileBytes
	refused := false
	// Enough sessions to pass 4 MiB even though each is held under 1 MiB.
	for s := 0; s < maxScratchPartitionBytes/maxSessionBytes+2 && !refused; s++ {
		ctx := WithSession(
			WithRequester(context.Background(), "111"),
			DirectSession(fmt.Sprintf("channel-%d", s), "111"),
		)
		session, err := provider.Open(ctx)
		if err != nil {
			t.Fatalf("open: %v", err)
		}
		for i := 0; i < perSession; i++ {
			result := callScratch(t, session, "scratch_write", map[string]any{
				"path": fmt.Sprintf("fill-%d.txt", i), "content": chunk,
			})
			if result.IsError {
				refused = true
				break
			}
		}
	}
	if !refused {
		t.Fatal("the per-requester ceiling never refused a write across sessions")
	}
}

// Overwriting must not count the old bytes twice, or a file could become
// unwritable by being rewritten at the same size.
func TestScratchOverwriteDoesNotDoubleCount(t *testing.T) {
	session := openScratch(t, t.TempDir(), "111")
	content := strings.Repeat("z", maxScratchFileBytes)
	for i := 0; i < 40; i++ {
		result := callScratch(t, session, "scratch_write", map[string]any{
			"path": "same.txt", "content": content,
		})
		if result.IsError {
			t.Fatalf("rewrite %d refused: %s", i, result.Text)
		}
	}
}

func TestScratchRejectsNonUTF8Content(t *testing.T) {
	session := openScratch(t, t.TempDir(), "111")
	result := callScratch(t, session, "scratch_write", map[string]any{
		"path": "bad.txt", "content": string([]byte{0xff, 0xfe}),
	})
	if !result.IsError {
		t.Fatal("non-UTF-8 content was accepted")
	}
}

// A written file must not be executable. Text only is enforced by the mode
// rather than described in a tool description.
func TestScratchWritesNonExecutableFiles(t *testing.T) {
	root := t.TempDir()
	session := openScratch(t, root, "111")
	callScratch(t, session, "scratch_write", map[string]any{
		"path": "script.sh", "content": "#!/bin/sh\necho hi\n",
	})
	info, err := os.Stat(filepath.Join(testPartition(root, "111"), "script.sh"))
	if err != nil {
		t.Fatalf("stat: %v", err)
	}
	if info.Mode().Perm()&0o111 != 0 {
		t.Fatalf("file is executable: %v", info.Mode().Perm())
	}
}

// A snowflake reaches the filesystem as a flat name and nothing else.
func TestScratchPartitionNameIsFlat(t *testing.T) {
	for _, requester := range []string{"../escape", "a/b", ".", "..", "with space"} {
		name := scratchPartitionName(requester)
		if strings.ContainsAny(name, `/\.`) || name == "" {
			t.Fatalf("partition name %q from %q is not flat", name, requester)
		}
	}
	// Only an absent requester falls back. A punctuation-only id is a distinct
	// requester and must get its own partition rather than the shared one.
	if scratchPartitionName("!!!") == "unattributed" {
		t.Fatal("a punctuation-only id fell back into the shared partition")
	}
	if scratchPartitionName("") != "unattributed" {
		t.Fatal("an absent requester did not fall back")
	}
}

// Telemetry reports Original, so a tool that leaves it unset is dispatched
// correctly and then recorded as an unnamed call.
func TestScratchToolsCarryOriginalName(t *testing.T) {
	session := openScratch(t, t.TempDir(), "111")
	tools := session.Tools()
	if len(tools) == 0 {
		t.Fatal("no tools offered")
	}
	for _, definition := range tools {
		if definition.Original == "" {
			t.Fatalf("%s has no Original, so its calls report an empty tool name", definition.Name)
		}
		if definition.Original != definition.Name {
			t.Fatalf("%s reports as %q", definition.Name, definition.Original)
		}
		if definition.Server == "" {
			t.Fatalf("%s has no Server", definition.Name)
		}
	}
}

// The composite backstops a provider that forgets, because the failure is a
// silent gap in telemetry rather than an error anybody sees.
func TestCompositeFillsMissingOriginalName(t *testing.T) {
	provider := &CompositeProvider{Providers: []ToolProvider{&unnamedToolProvider{}}}
	session, err := provider.Open(context.Background())
	if err != nil {
		t.Fatalf("open composite: %v", err)
	}
	t.Cleanup(func() { _ = session.Close() })
	tools := session.Tools()
	if len(tools) != 1 || tools[0].Original != "forgetful" {
		t.Fatalf("composite did not fill Original: %+v", tools)
	}
}

type unnamedToolProvider struct{}

func (p *unnamedToolProvider) Open(context.Context) (ToolSession, error) {
	return &unnamedToolSession{}, nil
}

type unnamedToolSession struct{}

func (s *unnamedToolSession) Tools() []ToolDefinition {
	return []ToolDefinition{{Name: "forgetful", Server: "test"}}
}

func (s *unnamedToolSession) Grounding() []GroundingDocument { return nil }
func (s *unnamedToolSession) Guidance() []ServerGuidance     { return nil }

func (s *unnamedToolSession) Unavailable() []string { return nil }

func (s *unnamedToolSession) Close() error { return nil }

func (s *unnamedToolSession) Call(
	context.Context,
	string,
	map[string]any,
) (ToolResult, error) {
	return ToolResult{}, nil
}

// The composite must not let two providers answer to one name.
func TestCompositeRefusesToolNameCollision(t *testing.T) {
	root := t.TempDir()
	provider := &CompositeProvider{Providers: []ToolProvider{
		&ScratchProvider{Root: root},
		&ScratchProvider{Root: root},
	}}
	if _, err := provider.Open(scratchContext("111")); err == nil {
		t.Fatal("composite accepted colliding tool names")
	}
}

func TestCompositeMergesToolsAndRoutesCalls(t *testing.T) {
	provider := &CompositeProvider{Providers: []ToolProvider{
		&ScratchProvider{},
		&ScratchProvider{Root: t.TempDir()},
	}}
	session, err := provider.Open(scratchContext("111"))
	if err != nil {
		t.Fatalf("open composite: %v", err)
	}
	t.Cleanup(func() { _ = session.Close() })
	if len(session.Tools()) != 4 {
		t.Fatalf("composite offered %d tools", len(session.Tools()))
	}
	result := callScratch(t, session, "scratch_write", map[string]any{
		"path": "a.txt", "content": "hello",
	})
	if result.IsError {
		t.Fatalf("composite refused a routed call: %s", result.Text)
	}
}

// The same requester must reach the same partition across turns, or a caller
// loses its own files.
func TestScratchPartitionIsStable(t *testing.T) {
	t.Parallel()
	first := scratchPartitionName("318190481467244544")
	if second := scratchPartitionName("318190481467244544"); first != second {
		t.Fatalf("partition drifted: %q then %q", first, second)
	}
}

// The identifier must not be recoverable from a directory listing, which is
// what the original function set out to do and did only partly.
func TestScratchPartitionHidesTheIdentifier(t *testing.T) {
	t.Parallel()
	const snowflake = "318190481467244544"
	name := scratchPartitionName(snowflake)
	if strings.Contains(name, snowflake) {
		t.Fatalf("partition %q carries the requester", name)
	}
	if strings.ContainsAny(name, "/.:\\ ") {
		t.Fatalf("partition %q is not a flat name", name)
	}
}

// An unattributed turn is refused before this is reached, but the empty case
// must still produce one safe name rather than an empty path.
func TestScratchPartitionHandlesAnEmptyRequester(t *testing.T) {
	t.Parallel()
	for _, requester := range []string{"", "   "} {
		if got := scratchPartitionName(requester); got != "unattributed" {
			t.Fatalf("empty requester produced %q", got)
		}
	}
}
