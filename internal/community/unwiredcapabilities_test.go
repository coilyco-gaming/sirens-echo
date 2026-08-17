package community

import (
	"regexp"
	"strings"
	"testing"
)

// Each of these has unit tests and no production caller, which is the pair
// sirens-echo#621 calls a green suite that reads as delivery.

// unwiredCapability is a function sirens-echo#621 lists as groundwork. The
// issue asks for each to be wired or deleted, with the reason recorded.
type unwiredCapability struct {
	name string
	does string
}

var unwiredCapabilities = []unwiredCapability{
	// Groundwork for sirens-echo#127, decided on sirens-echo#823. Reasons in
	// docs/sirens-echo-prompt-commands.md.
	{"CommandFromPrompt", "renders an MCP prompt as a Discord command"},
}

// RecordEffect and EffectApplied left this table when the job content path
// wired them. See sirens-echo#621 and docs/sirens-echo-jobs.md.

// IsGrantDenial left it when both submit surfaces started classifying a
// refusal. See sirens-echo#825 and docs/sirens-echo-grants.md.

// declared reports whether the function still exists. Both have unit tests,
// so this only fires on a deletion that took those tests with it.
func declared(t *testing.T, name string) bool {
	t.Helper()
	declaration := regexp.MustCompile(`^func (?:\([^)]*\) )?` + name + `\(`)
	for _, body := range nonTestGoSources(t) {
		for _, line := range strings.Split(body, "\n") {
			if declaration.MatchString(strings.TrimSpace(line)) {
				return true
			}
		}
	}
	return false
}

// Both outcomes sirens-echo#621 accepts are failures here, because both are
// decisions somebody has to write down rather than leave to a caller count.
func TestTheUnwiredCapabilitiesAreStillUnwired(t *testing.T) {
	t.Parallel()
	for _, capability := range unwiredCapabilities {
		if !declared(t, capability.name) {
			t.Errorf("%s is gone, so it was deleted rather than wired. Record "+
				"the reason on sirens-echo#621 and drop this row", capability.name)
			continue
		}
		if callers := productionCallers(t, capability.name); callers > 0 {
			t.Errorf("%s (%s) now has %d production caller(s). sirens-echo#621 "+
				"asks that wiring be recorded, so document it and drop this row",
				capability.name, capability.does, callers)
		}
	}
}

// promptcommand.go is unreachable as a unit: its only internal helper is called
// from nowhere else, so a caller count on the entry point understates nothing.
func TestThePromptCommandFileIsReachableOnlyFromItself(t *testing.T) {
	t.Parallel()
	if callers := productionCallers(t, "commandNameFor"); callers == 0 {
		t.Error("commandNameFor lost its in-file callers, so promptcommand.go " +
			"changed shape and sirens-echo#621's reasoning needs re-reading")
	}
	for _, body := range nonTestGoSources(t) {
		if strings.Contains(body, "commandNameFor") &&
			!strings.Contains(body, "func commandNameFor(") {
			t.Error("commandNameFor is now called from another file, so the " +
				"file is no longer unreachable as a unit. See sirens-echo#621")
		}
	}
}
