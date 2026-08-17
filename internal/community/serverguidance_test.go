package community

import (
	"strings"
	"testing"
)

// A server's own instructions are what tell the model which surface answers a
// request. See sirens-echo#647 and docs/sirens-echo-mcp.md.

func TestGuidanceMessageNamesEachSurface(t *testing.T) {
	t.Parallel()
	rendered := guidanceMessage([]ServerGuidance{
		{Server: "eco", Text: "Live Eco server state: players, trades, stores."},
		{Server: "forgejo", Text: "Read-only issue and repository lookups."},
	})
	for _, want := range []string{"eco", "forgejo", "Live Eco server state", "issue and repository"} {
		if !strings.Contains(rendered, want) {
			t.Errorf("rendered guidance is missing %q:\n%s", want, rendered)
		}
	}
}

// A server describing itself is not a server granting itself authority, so the
// framing says so in the same message.
func TestGuidanceIsFramedAsDescriptionNotAuthority(t *testing.T) {
	t.Parallel()
	rendered := guidanceMessage([]ServerGuidance{{Server: "eco", Text: "anything"}})
	if !strings.Contains(rendered, "does not grant authority") {
		t.Errorf("guidance reached the prompt without bounding what it is:\n%s", rendered)
	}
}

// Absence is nothing, not a heading with nothing under it. A section saying a
// surface published no description is the fail-open the port was rejected for.
func TestNoGuidanceRendersNothing(t *testing.T) {
	t.Parallel()
	if got := guidanceMessage(nil); got != "" {
		t.Errorf("guidanceMessage(nil) = %q, want empty", got)
	}
	if got := guidanceMessage([]ServerGuidance{}); got != "" {
		t.Errorf("guidanceMessage(empty) = %q, want empty", got)
	}
}

// A server that publishes nothing contributes nothing, rather than an entry
// whose text is blank.
func TestABlankInstructionIsNotAnEntry(t *testing.T) {
	t.Parallel()
	for _, raw := range []string{"", "   ", "\n\t "} {
		if text, ok := boundGuidanceText(raw); ok {
			t.Errorf("boundGuidanceText(%q) = (%q, true), want absent", raw, text)
		}
	}
	if _, ok := serverGuidance("eco", nil); ok {
		t.Error("a nil session produced guidance")
	}
}

// A server writes this text, so it is bounded like every other server-supplied
// string that reaches the prompt.
func TestGuidanceIsBounded(t *testing.T) {
	t.Parallel()
	text, ok := boundGuidanceText(strings.Repeat("x", maxServerGuidanceBytes*2))
	if !ok {
		t.Fatal("a long instruction was dropped rather than bounded")
	}
	if len(text) > maxServerGuidanceBytes {
		t.Errorf("bounded text is %d bytes, over the %d bound", len(text), maxServerGuidanceBytes)
	}
	if !strings.HasSuffix(text, "…") {
		t.Error("a truncated instruction does not say it was truncated")
	}
}

// Ordinary text passes through unchanged, so the bound cannot quietly reshape
// what a server said about itself.
func TestOrdinaryGuidancePassesThrough(t *testing.T) {
	t.Parallel()
	const said = "Live Eco server state: players, trades, stores."
	text, ok := boundGuidanceText("  " + said + "  ")
	if !ok || text != said {
		t.Errorf("boundGuidanceText = (%q, %v), want (%q, true)", text, ok, said)
	}
}
