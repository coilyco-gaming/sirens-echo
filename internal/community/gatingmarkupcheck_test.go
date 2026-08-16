package community

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// forbid_tool_call_markup is set on 24 cases across the rate packs and on none
// of the packs that gate a deployment. See sirens-echo#301.

// gatingPack is a pack whose result decides whether a change ships, with the
// number of its cases that ask for the tool-call markup check today.
type gatingPack struct {
	file     string
	cases    int
	guarding int
}

// The counts are asserted rather than described, so the gap cannot close or
// widen without someone reading sirens-echo#301.
var gatingPacks = []gatingPack{
	{"agents/echo/packs/evaluation.yaml", 10, 0},
	{"agents/deep/packs/evaluation.yaml", 9, 0},
	{"agents/deep/packs/board.yaml", 10, 0},
}

func countInPack(t *testing.T, name string) (cases, guarding int) {
	t.Helper()
	body, err := os.ReadFile(filepath.Join("..", "..", name))
	if err != nil {
		t.Fatalf("read %s: %v", name, err)
	}
	for _, line := range strings.Split(string(body), "\n") {
		trimmed := strings.TrimSpace(line)
		if strings.HasPrefix(line, "  - id:") {
			cases++
		}
		if strings.HasPrefix(trimmed, "forbid_tool_call_markup:") &&
			strings.Contains(trimmed, "true") {
			guarding++
		}
	}
	return cases, guarding
}

// A reply carrying the model's own tool-call syntax is never correct, and the
// check that says so is not asked for by any pack that gates a deployment.
func TestNoGatingPackAsksForTheToolCallMarkupCheck(t *testing.T) {
	t.Parallel()
	for _, pack := range gatingPacks {
		cases, guarding := countInPack(t, pack.file)
		if cases != pack.cases {
			t.Errorf("agent/%s now has %d cases, was %d. Recount its guarding "+
				"cases and update this row", pack.file, cases, pack.cases)
		}
		if guarding == pack.guarding {
			continue
		}
		if guarding > pack.guarding {
			t.Errorf("agent/%s now asks for the markup check on %d of %d cases, "+
				"was %d. If sirens-echo#301 is being closed, update this row; when "+
				"every gating pack covers every case, delete this test",
				pack.file, guarding, cases, pack.guarding)
			continue
		}
		t.Errorf("agent/%s dropped the markup check to %d of %d cases, was %d. "+
			"A gate that stops looking is worse than one that never did. "+
			"See sirens-echo#301", pack.file, guarding, cases, pack.guarding)
	}
}

// The check itself is built and used, so sirens-echo#301 is configuration
// rather than code. This fails if the rate packs stop exercising it.
func TestTheRatePacksDoAskForIt(t *testing.T) {
	t.Parallel()
	guarding := 0
	for _, name := range []string{
		"agents/echo/packs/rate.yaml", "agents/deep/packs/rate.yaml",
		"agents/deep/packs/rate-fixture.yaml",
		"agent/rate-fixture-tracker.yaml", "agent/rate-fixture-tracker-match.yaml",
	} {
		_, count := countInPack(t, name)
		guarding += count
	}
	if guarding == 0 {
		t.Error("no rate pack asks for the tool-call markup check any more, so " +
			"the only cases exercising it are gone. See sirens-echo#301")
	}
}
