package community

import (
	"testing"
	"time"
)

// A deployment may tune the timeouts. It may not tune a security floor, and a
// derived value must not be left behind. See sirens-echo#660.

func fixedLookup(pairs map[string]string) func(string) string {
	return func(name string) string { return pairs[name] }
}

func TestAnOverrideApplies(t *testing.T) {
	original := defaultRequestTimeout
	defer func() { defaultRequestTimeout = original }()
	applied := applyTuningOverrides(fixedLookup(map[string]string{
		"SIRENS_ECHO_REQUEST_TIMEOUT": "90s",
	}))
	if defaultRequestTimeout != 90*time.Second {
		t.Errorf("request timeout = %s, want 90s", defaultRequestTimeout)
	}
	if len(applied) != 1 || applied[0] != "SIRENS_ECHO_REQUEST_TIMEOUT" {
		t.Errorf("applied = %v, want the one name set", applied)
	}
}

// The trap: moving the beat and leaving the threshold is an override that
// half-works in silence.
func TestTheDerivedPairIsRecomputed(t *testing.T) {
	originalAfter := turnProgressAfter
	originalEvery := turnProgressEvery
	originalLong := turnLongReplyAfter
	defer func() {
		turnProgressAfter, turnProgressEvery, turnLongReplyAfter =
			originalAfter, originalEvery, originalLong
	}()
	applyTuningOverrides(fixedLookup(map[string]string{
		"SIRENS_ECHO_PROGRESS_AFTER": "2s",
	}))
	if turnProgressEvery != 4*time.Second {
		t.Errorf("beat = %s, want 4s: the derived value was not recomputed", turnProgressEvery)
	}
	if turnLongReplyAfter != 10*time.Second {
		t.Errorf("threshold = %s, want 10s", turnLongReplyAfter)
	}
}

// A typo keeps the default rather than applying a number nobody chose, which
// is the failure this repository has hit repeatedly.
func TestAMalformedOrNonPositiveValueIsIgnored(t *testing.T) {
	original := defaultRequestTimeout
	defer func() { defaultRequestTimeout = original }()
	for name, raw := range map[string]string{
		"not a duration": "soon",
		"bare number":    "90",
		"zero":           "0s",
		"negative":       "-30s",
		"empty":          "",
	} {
		defaultRequestTimeout = original
		applied := applyTuningOverrides(fixedLookup(map[string]string{
			"SIRENS_ECHO_REQUEST_TIMEOUT": raw,
		}))
		if defaultRequestTimeout != original {
			t.Errorf("%s (%q) changed the timeout to %s", name, raw, defaultRequestTimeout)
		}
		for _, got := range applied {
			if got == "SIRENS_ECHO_REQUEST_TIMEOUT" {
				t.Errorf("%s (%q) reported itself applied", name, raw)
			}
		}
	}
}

// The carve-out. A security floor must not be reachable from a values file.
func TestNoSecurityFloorIsOverridable(t *testing.T) {
	t.Parallel()
	for _, name := range []string{
		"SIRENS_ECHO_OPAQUE_SECRET_RUNES",
		"SIRENS_ECHO_MIN_ENCODED_GUARD_BYTES",
		"SIRENS_ECHO_MAX_PROXY_TOOL_NAME_BYTES",
	} {
		if _, present := tuningOverrides()[name]; present {
			t.Errorf("%s is overridable, which lets a values file loosen a guard", name)
		}
	}
}
