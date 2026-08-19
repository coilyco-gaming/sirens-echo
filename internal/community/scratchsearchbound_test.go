package community

import (
	"strings"
	"testing"
)

// scratch_search was bounded by a match count and not by bytes, so a hundred
// long lines answered with 131KB against a 16KB consumer. See sirens-echo#940.

func TestSearchStopsOnBytesRatherThanOnlyMatches(t *testing.T) {
	restoreKnobs(t)
	root := t.TempDir()
	session := openScratch(t, root, "ana")
	// Under the 100-match cap, so a count-only bound would return every one of
	// these at full length, which is what produced the 131KB result.
	long := strings.Repeat("needle ", 400)
	body := strings.TrimRight(strings.Repeat(long+"\n", 6), "\n")
	for _, name := range []string{"a.txt", "b.txt", "c.txt", "d.txt", "e.txt",
		"f.txt", "g.txt", "h.txt", "i.txt", "j.txt"} {
		write := callScratch(t, session, "scratch_write", map[string]any{
			"path": name, "content": body,
		})
		if write.IsError {
			t.Fatalf("write refused: %s", write.Text)
		}
	}
	found := callScratch(t, session, "scratch_search", map[string]any{"query": "needle"})
	if found.IsError {
		t.Fatalf("search refused: %s", found.Text)
	}
	// The bound the consumer actually has. Before this the same corpus
	// answered with an order of magnitude more.
	if len(found.Text) > maxToolResultBytes*2 {
		t.Fatalf("search returned %d bytes against a %d byte tool budget",
			len(found.Text), maxToolResultBytes)
	}
	if !strings.Contains(found.Text, "stopped at") {
		t.Errorf("a cut result does not say it was cut: %q", found.Text[:200])
	}
}

// A minified file is one line, so a single hit could be the whole answer.
func TestOneLongMatchCannotBeTheWholeResult(t *testing.T) {
	restoreKnobs(t)
	root := t.TempDir()
	session := openScratch(t, root, "ana")
	write := callScratch(t, session, "scratch_write", map[string]any{
		"path": "min.js", "content": "needle" + strings.Repeat("x", 20000),
	})
	if write.IsError {
		t.Fatalf("write refused: %s", write.Text)
	}
	found := callScratch(t, session, "scratch_search", map[string]any{"query": "needle"})
	if len(found.Text) > maxScratchMatchRunes*2 {
		t.Fatalf("one match rendered %d bytes against a %d rune line bound",
			len(found.Text), maxScratchMatchRunes)
	}
	if !strings.Contains(found.Text, "...") {
		t.Errorf("a cut line does not show it was cut: %q", found.Text)
	}
}

// The ordinary case has to keep working, or the bound has bought nothing.
func TestASmallSearchIsUnchangedAndSaysNothingAboutTruncation(t *testing.T) {
	restoreKnobs(t)
	root := t.TempDir()
	session := openScratch(t, root, "ana")
	write := callScratch(t, session, "scratch_write", map[string]any{
		"path": "notes.md", "content": "one\nneedle here\nthree",
	})
	if write.IsError {
		t.Fatalf("write refused: %s", write.Text)
	}
	found := callScratch(t, session, "scratch_search", map[string]any{"query": "needle"})
	if !strings.Contains(found.Text, "notes.md:2: needle here") {
		t.Fatalf("the match is missing or reshaped: %q", found.Text)
	}
	if strings.Contains(found.Text, "stopped at") {
		t.Errorf("a complete result claimed it was cut: %q", found.Text)
	}
	if strings.Contains(found.Text, "...") {
		t.Errorf("a short line was trimmed: %q", found.Text)
	}
}
