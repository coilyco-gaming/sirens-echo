package community

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// Every decision on sirens-echo#156 that a reader would want proved rather
// than described.

func sessionScratch(t *testing.T, root, requester string, id SessionID) ToolSession {
	t.Helper()
	provider := &ScratchProvider{Root: root}
	session, err := provider.Open(WithSession(
		WithRequester(context.Background(), requester), id,
	))
	if err != nil {
		t.Fatalf("open scratchpad: %v", err)
	}
	return session
}

// Two members in one thread share a workspace, which is the whole reason the
// partition is a session rather than a requester.
func TestAThreadIsOneWorkspaceForEveryone(t *testing.T) {
	root := t.TempDir()
	thread := ThreadSession("thread-1")

	author := sessionScratch(t, root, "111", thread)
	wrote := callScratch(t, author, "scratch_write", map[string]any{
		"path": "plan.md", "content": "the shared plan",
	})
	if wrote.IsError {
		t.Fatalf("write: %s", wrote.Text)
	}

	// A different member, same thread.
	reader := sessionScratch(t, root, "222", thread)
	listed := callScratch(t, reader, "scratch_list", nil)
	if !strings.Contains(listed.Text, "plan.md") {
		t.Fatalf("the other member could not see the file: %s", listed.Text)
	}
	found := callScratch(t, reader, "scratch_search", map[string]any{"query": "shared plan"})
	if found.IsError || !strings.Contains(found.Text, "plan.md") {
		t.Fatalf("search did not span the session: %s", found.Text)
	}
}

// Outside a thread there is no bounded conversation, so two members in one
// channel must not read each other's files.
func TestAChannelPairingIsPrivate(t *testing.T) {
	root := t.TempDir()
	author := sessionScratch(t, root, "111", DirectSession("channel-1", "111"))
	callScratch(t, author, "scratch_write", map[string]any{
		"path": "private.md", "content": "mine alone",
	})

	other := sessionScratch(t, root, "222", DirectSession("channel-1", "222"))
	listed := callScratch(t, other, "scratch_list", nil)
	if strings.Contains(listed.Text, "private.md") {
		t.Fatalf("a channel pairing leaked into another member's session: %s", listed.Text)
	}
}

// A bare path is your own file. Reading through the session must not turn
// `notes.txt` into somebody else's.
func TestYourOwnPathWinsOverTheSharedOne(t *testing.T) {
	root := t.TempDir()
	thread := ThreadSession("thread-2")
	mine := sessionScratch(t, root, "111", thread)
	theirs := sessionScratch(t, root, "222", thread)

	callScratch(t, mine, "scratch_write", map[string]any{"path": "notes.txt", "content": "mine"})
	callScratch(t, theirs, "scratch_write", map[string]any{"path": "notes.txt", "content": "theirs"})

	read := callScratch(t, mine, "scratch_read", map[string]any{"path": "notes.txt"})
	if read.IsError || !strings.Contains(read.Text, "mine") {
		t.Fatalf("a bare path did not resolve to the caller's own file: %s", read.Text)
	}
}

// The session cap evicts rather than refusing, so an active thread stays
// usable, and it names what it removed.
func TestTheSessionCapEvictsOldestFirst(t *testing.T) {
	root := t.TempDir()
	session := sessionScratch(t, root, "111", ThreadSession("thread-3"))
	chunk := strings.Repeat("z", maxScratchFileBytes)

	first := callScratch(t, session, "scratch_write", map[string]any{
		"path": "oldest.txt", "content": chunk,
	})
	if first.IsError {
		t.Fatalf("first write: %s", first.Text)
	}
	// Distinct modification times, so oldest-first has something to order by.
	time.Sleep(10 * time.Millisecond)

	evicted := false
	for i := 0; i < maxSessionBytes/maxScratchFileBytes+2; i++ {
		result := callScratch(t, session, "scratch_write", map[string]any{
			"path": filepath.Join("fill", string(rune('a'+i))+".txt"), "content": chunk,
		})
		if result.IsError {
			t.Fatalf("the session cap refused instead of evicting: %s", result.Text)
		}
		if strings.Contains(result.Text, "evicted") {
			evicted = true
			if !strings.Contains(result.Text, "oldest.txt") {
				t.Errorf("eviction did not take the oldest file first: %s", result.Text)
			}
			break
		}
	}
	if !evicted {
		t.Fatal("the session cap never evicted")
	}
}

// Eviction stays inside the session that overflowed, so one heavy thread
// cannot clear another conversation's files.
func TestEvictionDoesNotReachAnotherSession(t *testing.T) {
	root := t.TempDir()
	quiet := sessionScratch(t, root, "111", ThreadSession("quiet"))
	callScratch(t, quiet, "scratch_write", map[string]any{"path": "keep.txt", "content": "keep me"})

	busy := sessionScratch(t, root, "111", ThreadSession("busy"))
	chunk := strings.Repeat("z", maxScratchFileBytes)
	for i := 0; i < maxSessionBytes/maxScratchFileBytes+2; i++ {
		callScratch(t, busy, "scratch_write", map[string]any{
			"path": filepath.Join("fill", string(rune('a'+i))+".txt"), "content": chunk,
		})
	}

	still := callScratch(t, quiet, "scratch_read", map[string]any{"path": "keep.txt"})
	if still.IsError {
		t.Fatalf("a busy session evicted a quiet one's file: %s", still.Text)
	}
}

// Both timers, proved by collection rather than by configuration. A policy
// that never fires is indistinguishable from none.
func TestCollectSessionsAppliesEachTimer(t *testing.T) {
	root := t.TempDir()
	now := time.Now()

	thread := filepath.Join(root, ThreadSession("old-thread").Directory())
	direct := filepath.Join(root, DirectSession("channel", "111").Directory())
	fresh := filepath.Join(root, ThreadSession("fresh-thread").Directory())
	for _, dir := range []string{thread, direct, fresh} {
		if err := os.MkdirAll(dir, scratchPermissions); err != nil {
			t.Fatalf("prepare %s: %v", dir, err)
		}
	}
	// A thread is collected after a week; a channel pairing after an hour, so
	// two hours expires one and not the other.
	stale := now.Add(-2 * time.Hour)
	writeAged(t, filepath.Join(thread, "note.txt"), stale)
	writeAged(t, filepath.Join(direct, "note.txt"), stale)
	writeAged(t, filepath.Join(fresh, "note.txt"), now)

	collected, err := CollectSessions(root, now)
	if err != nil {
		t.Fatalf("collect: %v", err)
	}
	if len(collected) != 1 || collected[0] != DirectSession("channel", "111").Directory() {
		t.Fatalf("collected = %v, want only the idle channel pairing", collected)
	}
	if _, err := os.Stat(thread); err != nil {
		t.Error("a thread was collected two hours in, well inside its week")
	}
	if _, err := os.Stat(direct); !os.IsNotExist(err) {
		t.Error("the idle channel pairing survived its hour")
	}
}

// A week-old thread does go, or the longer timer is a timer that never fires.
func TestAQuietThreadIsCollectedAfterItsWeek(t *testing.T) {
	root := t.TempDir()
	now := time.Now()
	thread := filepath.Join(root, ThreadSession("ancient").Directory())
	if err := os.MkdirAll(thread, scratchPermissions); err != nil {
		t.Fatalf("prepare: %v", err)
	}
	writeAged(t, filepath.Join(thread, "note.txt"), now.Add(-8*24*time.Hour))

	collected, err := CollectSessions(root, now)
	if err != nil {
		t.Fatalf("collect: %v", err)
	}
	if len(collected) != 1 {
		t.Fatalf("collected = %v, want the quiet thread", collected)
	}
}

// A directory this scheme did not write is left alone, so the collector cannot
// remove something it does not understand.
func TestCollectSessionsLeavesForeignDirectories(t *testing.T) {
	root := t.TempDir()
	foreign := filepath.Join(root, "not-a-session")
	if err := os.MkdirAll(foreign, scratchPermissions); err != nil {
		t.Fatalf("prepare: %v", err)
	}
	writeAged(t, filepath.Join(foreign, "note.txt"), time.Now().Add(-30*24*time.Hour))

	collected, err := CollectSessions(root, time.Now())
	if err != nil {
		t.Fatalf("collect: %v", err)
	}
	if len(collected) != 0 {
		t.Fatalf("collected = %v, want nothing", collected)
	}
	if _, err := os.Stat(foreign); err != nil {
		t.Error("the collector removed a directory it did not write")
	}
}

// The kind has to survive the round trip to disk, since the sweeper reads the
// retention rule off the directory name.
func TestASessionDirectoryCarriesItsKind(t *testing.T) {
	for _, row := range []struct {
		id   SessionID
		want SessionKind
	}{
		{ThreadSession("a"), SessionThread},
		{DirectSession("c", "u"), SessionDirect},
	} {
		kind, known := sessionKindOf(row.id.Directory())
		if !known || kind != row.want {
			t.Errorf("%s read back as %q (known=%v), want %q",
				row.id.Directory(), kind, known, row.want)
		}
	}
}

// No member identifier may reach a path, because the model reads these.
func TestASessionDirectoryHidesItsIdentifiers(t *testing.T) {
	id := DirectSession("channel-999", "member-12345")
	if strings.Contains(id.Directory(), "12345") || strings.Contains(id.Directory(), "999") {
		t.Fatalf("a member identifier reached the path: %s", id.Directory())
	}
}

// writeAged writes a file and backdates it, which is how both timers are
// exercised without waiting.
func writeAged(t *testing.T, path string, when time.Time) {
	t.Helper()
	if err := os.WriteFile(path, []byte("x"), scratchFilePermissions); err != nil {
		t.Fatalf("write %s: %v", path, err)
	}
	if err := os.Chtimes(path, when, when); err != nil {
		t.Fatalf("age %s: %v", path, err)
	}
}

// An unmounted scratchpad wrote into the working directory, because an empty
// root confines to ".". It was inert only because measuring it errored first.
func TestAnUnmountedScratchpadWritesNothing(t *testing.T) {
	provider := &ScratchProvider{}
	session, err := provider.Open(context.Background())
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	writer, ok := session.(reservedWriter)
	if !ok {
		t.Fatal("the disabled session stopped offering the runtime writer")
	}
	result, err := writer.WriteReserved("tool-output/probe.txt", "content")
	if err != nil {
		t.Fatalf("WriteReserved: %v", err)
	}
	if !result.IsError {
		t.Fatal("an unmounted scratchpad accepted a write")
	}
	if _, err := os.Stat("tool-output"); !os.IsNotExist(err) {
		t.Fatal("an unmounted scratchpad wrote into the working directory")
	}
}
