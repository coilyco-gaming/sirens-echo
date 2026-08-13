package community

import (
	"strings"
	"testing"
)

// A nameless tool used to register under the server's own name, because
// trimming the separator left that half standing. See sirens-echo#587.

func TestANamelessToolIsRefused(t *testing.T) {
	t.Parallel()
	for name, pair := range map[string][2]string{
		"no tool":     {"forgejo", ""},
		"blank tool":  {"forgejo", "   "},
		"no server":   {"", "get_issue"},
		"neither":     {"", ""},
		"both blank":  {" ", "\t"},
		"tabbed tool": {"eco", "\n"},
	} {
		got, err := proxyToolName(pair[0], pair[1])
		if err == nil {
			t.Errorf("%s registered as %q instead of being refused", name, got)
		}
		if got != "" {
			t.Errorf("%s returned a usable name %q alongside its error", name, got)
		}
	}
}

// The exact shape that motivated this: the composed name is non-empty after
// trimming, so the old emptiness check could not see the missing half.
func TestTheServerNameAloneIsNotAToolName(t *testing.T) {
	t.Parallel()
	if _, err := proxyToolName("forgejo", ""); err == nil {
		t.Fatal("a nameless forgejo tool was accepted")
	}
	// Demonstrates why: composing and trimming yields the bare server name,
	// which the old check read as usable.
	composed := strings.Trim(
		invalidProxyToolName.ReplaceAllString("forgejo"+"__"+"", "_"), "_",
	)
	if composed != "forgejo" {
		t.Fatalf("composition changed: %q, so this test no longer pins the cause", composed)
	}
}

// Real tools must keep working, or the guard traded one defect for a worse one.
func TestOrdinaryToolNamesStillCompose(t *testing.T) {
	t.Parallel()
	for _, pair := range [][2]string{
		{"forgejo", "get_issue"},
		{"eco", "get_server_status"},
		{"scratchpad", "scratch_list"},
	} {
		got, err := proxyToolName(pair[0], pair[1])
		if err != nil {
			t.Errorf("proxyToolName(%q, %q): %v", pair[0], pair[1], err)
		}
		if want := pair[0] + "__" + pair[1]; got != want {
			t.Errorf("proxyToolName(%q, %q) = %q, want %q", pair[0], pair[1], got, want)
		}
	}
}
