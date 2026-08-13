package community

import (
	"strings"
	"testing"
	"unicode/utf8"
)

// runeBoundary bounds a tool result before it re-enters the prompt. A boundary
// landing mid-rune would feed the model invalid UTF-8, so the property is total.

// mixedWidths carries one, two, three, and four byte runes, so a walk back from
// an arbitrary offset crosses every width.
const mixedWidths = "a" + "é" + "€" + "🙂" + "b" + "漢字"

// Every offset is checked rather than a sample, because the failure is a single
// byte in the wrong place and the whole input is small enough to exhaust.
func TestRuneBoundaryNeverSplitsARune(t *testing.T) {
	t.Parallel()
	for limit := 0; limit <= len(mixedWidths); limit++ {
		boundary := runeBoundary(mixedWidths, limit)
		if boundary < 0 || boundary > limit {
			t.Fatalf("limit %d: boundary %d is out of range", limit, boundary)
		}
		if !utf8.ValidString(mixedWidths[:boundary]) {
			t.Errorf("limit %d: prefix of %d bytes is not valid UTF-8", limit, boundary)
		}
	}
}

// A limit past the end returns the whole value rather than growing it, which is
// what lets the caller slice unconditionally.
func TestRuneBoundaryClampsPastTheEnd(t *testing.T) {
	t.Parallel()
	for _, limit := range []int{len(mixedWidths), len(mixedWidths) + 1, len(mixedWidths) * 2} {
		if got := runeBoundary(mixedWidths, limit); got != len(mixedWidths) {
			t.Errorf("limit %d: boundary %d, want %d", limit, got, len(mixedWidths))
		}
	}
	if got := runeBoundary("", 5); got != 0 {
		t.Errorf("empty value: boundary %d, want 0", got)
	}
}

// A boundary inside a multibyte rune walks back to its start rather than
// forward, so the result is never longer than the caller asked for.
func TestRuneBoundaryWalksBackNotForward(t *testing.T) {
	t.Parallel()
	// Four bytes, one rune. Every interior offset must collapse to zero.
	emoji := "🙂"
	for limit := 1; limit < len(emoji); limit++ {
		if got := runeBoundary(emoji, limit); got != 0 {
			t.Errorf("limit %d inside a 4-byte rune: boundary %d, want 0", limit, got)
		}
	}
	// An ASCII prefix keeps its bytes, and the trailing partial rune is dropped.
	value := "ab🙂"
	for limit := 2; limit < len(value); limit++ {
		if got := runeBoundary(value, limit); got != 2 {
			t.Errorf("limit %d after two ASCII bytes: boundary %d, want 2", limit, got)
		}
	}
}

// Characterization. A negative limit is returned unchanged, so the caller's
// slice panics. No caller passes one; a computed budget would find it here.
func TestRuneBoundaryReturnsANegativeLimitUnchanged(t *testing.T) {
	t.Parallel()
	if got := runeBoundary(mixedWidths, -1); got != -1 {
		t.Errorf("negative limit returned %d; if it was clamped, assert the clamp", got)
	}
}

// The caller truncates a large result, so the boundary has to hold at the size
// the tool budget actually uses rather than only on short strings.
func TestRuneBoundaryHoldsAtToolResultScale(t *testing.T) {
	t.Parallel()
	large := strings.Repeat("漢", 4000)
	for _, limit := range []int{8 << 10, (8 << 10) - 1, (8 << 10) + 1} {
		boundary := runeBoundary(large, limit)
		if boundary > limit {
			t.Errorf("limit %d: boundary %d exceeds it", limit, boundary)
		}
		if !utf8.ValidString(large[:boundary]) {
			t.Errorf("limit %d: prefix is not valid UTF-8", limit)
		}
	}
}
