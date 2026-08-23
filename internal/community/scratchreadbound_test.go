package community

import (
	"context"
	"strconv"
	"strings"
	"testing"
)

// scratchWith puts one file in a session's scratchpad and returns the session.
func scratchWith(t *testing.T, name, content string) ToolSession {
	t.Helper()
	session := openScratch(t, t.TempDir(), "member-1")
	if _, err := session.Call(context.Background(), "scratch_write", map[string]any{
		"path": name, "content": content,
	}); err != nil {
		t.Fatalf("write: %v", err)
	}
	return session
}

func readAt(t *testing.T, session ToolSession, name string, offset int) ToolResult {
	t.Helper()
	arguments := map[string]any{"path": name}
	if offset > 0 {
		arguments["offset"] = float64(offset)
	}
	result, err := session.Call(context.Background(), "scratch_read", arguments)
	if err != nil {
		t.Fatalf("read at %d: %v", offset, err)
	}
	if result.IsError {
		t.Fatalf("read at %d refused: %s", offset, result.Text)
	}
	return result
}

// A 53KB read against a 16KB consumer had 70% dropped downstream, then cost a
// further round chasing the spill file. See sirens-echo#940.
func TestALongScratchReadComesBackBounded(t *testing.T) {
	// Not parallel. applyKnobs writes the package defaults every other test
	// reads, which is why no knob test takes t.Parallel.
	restoreKnobs(t)
	applyKnobs(func(string) string { return "" })

	long := strings.Repeat("a line of perfectly ordinary scratchpad text\n", 4000)
	session := scratchWith(t, "notes.txt", long)

	result := readAt(t, session, "notes.txt", 0)
	if len(result.Text) > maxToolResultBytes*2 {
		t.Errorf(
			"a %d byte file came back as %d bytes against a %d byte budget",
			len(long), len(result.Text), maxToolResultBytes,
		)
	}
	if !strings.Contains(result.Text, "read again with offset") {
		t.Errorf("the bounded piece does not say how to get the rest: %q", readTail(result.Text))
	}
	if !strings.Contains(result.Text, strconv.Itoa(len(long))) {
		t.Errorf("the bounded piece does not say how long the file is: %q", readTail(result.Text))
	}
}

// A short file must not grow a boundary note, which would read as a truncation
// that did not happen.
func TestAShortScratchReadIsUntouched(t *testing.T) {
	// Not parallel. applyKnobs writes the package defaults every other test
	// reads, which is why no knob test takes t.Parallel.
	restoreKnobs(t)
	applyKnobs(func(string) string { return "" })

	const short = "three lines\nof nothing\nmuch at all\n"
	session := scratchWith(t, "short.txt", short)

	if got := readAt(t, session, "short.txt", 0).Text; got != short {
		t.Errorf("read = %q, want the file unchanged", got)
	}
}

// The offset has to actually continue, or the note sends the model somewhere
// that repeats or skips content.
func TestReadingFromTheReportedOffsetReassemblesTheFile(t *testing.T) {
	// Not parallel. applyKnobs writes the package defaults every other test
	// reads, which is why no knob test takes t.Parallel.
	restoreKnobs(t)
	applyKnobs(func(string) string { return "" })

	long := strings.Repeat("a line of perfectly ordinary scratchpad text\n", 4000)
	session := scratchWith(t, "notes.txt", long)

	var rebuilt strings.Builder
	offset := 0
	for pieces := 0; pieces < 64; pieces++ {
		text := readAt(t, session, "notes.txt", offset).Text
		body, next, more := splitBoundaryNote(t, text)
		rebuilt.WriteString(body)
		if !more {
			break
		}
		if next <= offset {
			t.Fatalf("offset went from %d to %d, so the read does not advance", offset, next)
		}
		offset = next
	}
	if rebuilt.String() != long {
		t.Errorf(
			"reassembled %d bytes from the reported offsets, want the %d byte file",
			rebuilt.Len(), len(long),
		)
	}
}

// splitBoundaryNote takes the piece apart into its content and the offset it
// reported, so the test reads what the model would read.
func splitBoundaryNote(t *testing.T, text string) (body string, next int, more bool) {
	t.Helper()
	const marker = "\n[notes.txt: bytes "
	at := strings.LastIndex(text, marker)
	if at < 0 {
		return text, 0, false
	}
	note := text[at+len(marker):]
	_, after, found := strings.Cut(note, "read again with offset ")
	if !found {
		t.Fatalf("the note carries no offset: %q", note)
	}
	digits, _, _ := strings.Cut(after, "]")
	parsed, err := strconv.Atoi(strings.TrimSpace(digits))
	if err != nil {
		t.Fatalf("offset %q is not a number: %v", digits, err)
	}
	return text[:at], parsed, true
}

// An offset past the end is a mistake worth naming, not an empty result that
// reads as an empty file.
func TestAnOffsetPastTheEndIsRefused(t *testing.T) {
	t.Parallel()
	session := scratchWith(t, "short.txt", "a few bytes\n")
	result, err := session.Call(context.Background(), "scratch_read", map[string]any{
		"path": "short.txt", "offset": float64(9000),
	})
	if err != nil {
		t.Fatalf("read: %v", err)
	}
	if !result.IsError {
		t.Errorf("an offset past the end returned %q rather than a refusal", result.Text)
	}
}

// readTail keeps a failure message readable when the piece is large.
func readTail(text string) string {
	if len(text) <= 200 {
		return text
	}
	return "..." + text[len(text)-200:]
}
