package community

import (
	"testing"
	"time"
)

// A deployment may tune every number this service ships. It may not reach an
// algorithm's floor, and a derived value must not be left behind.

func fixedLookup(pairs map[string]string) func(string) string {
	return func(name string) string { return pairs[name] }
}

// restoreKnobs puts every number back on its default, so one test's override
// cannot become another test's starting state.
func restoreKnobs(t *testing.T) {
	t.Helper()
	t.Cleanup(func() { applyKnobs(func(string) string { return "" }) })
}

func TestADurationOverrideApplies(t *testing.T) {
	restoreKnobs(t)
	applied, rejected := applyKnobs(fixedLookup(map[string]string{
		"SIRENS_ECHO_REQUEST_TIMEOUT": "90s",
	}))
	if defaultRequestTimeout != 90*time.Second {
		t.Errorf("request timeout = %s, want 90s", defaultRequestTimeout)
	}
	if len(applied) != 1 || applied[0] != "SIRENS_ECHO_REQUEST_TIMEOUT" {
		t.Errorf("applied = %v, want the one name set", applied)
	}
	if len(rejected) != 0 {
		t.Errorf("rejected = %v, want none", rejected)
	}
}

// The half Kai asked for: a count is as settable as a timeout, through the
// same helper and the same table.
func TestACountOverrideApplies(t *testing.T) {
	restoreKnobs(t)
	applied, _ := applyKnobs(fixedLookup(map[string]string{
		"SIRENS_ECHO_TOOL_ROUNDS":     "3",
		"SIRENS_ECHO_SCRATCH_ENTRIES": "42",
	}))
	if maxToolRounds != 3 {
		t.Errorf("tool rounds = %d, want 3", maxToolRounds)
	}
	if maxScratchEntries != 42 {
		t.Errorf("scratch entries = %d, want 42", maxScratchEntries)
	}
	if len(applied) != 2 {
		t.Errorf("applied = %v, want both names", applied)
	}
}

// The trap: moving the beat and leaving the threshold is an override that
// half-works in silence.
func TestTheDerivedValuesAreRecomputed(t *testing.T) {
	restoreKnobs(t)
	applyKnobs(fixedLookup(map[string]string{
		"SIRENS_ECHO_PROGRESS_AFTER":     "2s",
		"SIRENS_ECHO_SCRATCH_FILE_BYTES": "4096",
	}))
	if turnProgressEvery != 4*time.Second {
		t.Errorf("beat = %s, want 4s: the derived value was not recomputed", turnProgressEvery)
	}
	if turnLongReplyAfter != 10*time.Second {
		t.Errorf("threshold = %s, want 10s", turnLongReplyAfter)
	}
	// The attachment bound follows the scratchpad's file limit, so setting the
	// limit is how it moves. Overriding it on its own is not offered.
	if replyAttachmentBytes != 4096 {
		t.Errorf("attachment bound = %d, want 4096", replyAttachmentBytes)
	}
}

// A typo keeps the default rather than applying a number nobody chose, and is
// named rather than swallowed.
func TestAMalformedOrNonPositiveValueIsRejectedAndNamed(t *testing.T) {
	restoreKnobs(t)
	applyKnobs(func(string) string { return "" })
	declared := defaultRequestTimeout
	for name, raw := range map[string]string{
		"not a duration": "soon",
		"bare number":    "90",
		"zero":           "0s",
		"negative":       "-30s",
	} {
		applied, rejected := applyKnobs(fixedLookup(map[string]string{
			"SIRENS_ECHO_REQUEST_TIMEOUT": raw,
		}))
		if defaultRequestTimeout != declared {
			t.Errorf("%s (%q) changed the timeout to %s", name, raw, defaultRequestTimeout)
		}
		for _, got := range applied {
			if got == "SIRENS_ECHO_REQUEST_TIMEOUT" {
				t.Errorf("%s (%q) reported itself applied", name, raw)
			}
		}
		if len(rejected) != 1 || rejected[0] != "SIRENS_ECHO_REQUEST_TIMEOUT" {
			t.Errorf("%s (%q) was not reported rejected: %v", name, raw, rejected)
		}
	}
}

// An unset name is not a rejection. Reporting one would make every deployment
// look misconfigured.
func TestAnUnsetNameIsNeitherAppliedNorRejected(t *testing.T) {
	restoreKnobs(t)
	applied, rejected := applyKnobs(fixedLookup(map[string]string{
		"SIRENS_ECHO_REQUEST_TIMEOUT": "   ",
	}))
	if len(applied) != 0 || len(rejected) != 0 {
		t.Errorf("applied = %v and rejected = %v, want neither", applied, rejected)
	}
}

// A second call must start from the defaults rather than from what the first
// one left behind, or a rejected value inherits an earlier override.
func TestApplyingTwiceIsNotCumulative(t *testing.T) {
	restoreKnobs(t)
	applyKnobs(func(string) string { return "" })
	declared := maxToolRounds
	applyKnobs(fixedLookup(map[string]string{"SIRENS_ECHO_TOOL_ROUNDS": "3"}))
	applyKnobs(fixedLookup(map[string]string{"SIRENS_ECHO_QUEUE_TIMEOUT": "9s"}))
	if maxToolRounds != declared {
		t.Errorf("tool rounds = %d, want the declared default %d back", maxToolRounds, declared)
	}
	if defaultQueueTimeout != 9*time.Second {
		t.Errorf("queue timeout = %s, want 9s", defaultQueueTimeout)
	}
}

// The carve-out that survives. An algorithm's floor decides what counts as a
// match rather than how much to allow, so no name reaches it.
func TestNoAlgorithmFloorIsOverridable(t *testing.T) {
	t.Parallel()
	named := make(map[string]bool)
	for _, entry := range knobs() {
		named[entry.env] = true
	}
	for _, name := range []string{
		"SIRENS_ECHO_OPAQUE_SECRET_RUNES",
		"SIRENS_ECHO_MIN_NORMALIZED_ID_DIGITS",
		"SIRENS_ECHO_MIN_ENCODED_GUARD_BYTES",
	} {
		if named[name] {
			t.Errorf("%s is overridable, which lets a values file loosen a guard", name)
		}
	}
}
