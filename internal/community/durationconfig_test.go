package community

import (
	"strings"
	"testing"
	"time"
)

// durationOrDefault decides the request timeout, the queue timeout, and the
// rate-limit notify window from configuration. Three deadlines, one parser.

// An absent value takes the packaged default, which is what lets a deployment
// set only the knobs it cares about.
func TestDurationOrDefaultFallsBackOnlyWhenAbsent(t *testing.T) {
	t.Parallel()
	const fallback = 90 * time.Second
	for _, value := range []string{"", "   ", "\t\n"} {
		got, err := durationOrDefault(value, fallback)
		if err != nil {
			t.Errorf("%q: %v", value, err)
		}
		if got != fallback {
			t.Errorf("%q: %s, want the fallback %s", value, got, fallback)
		}
	}
}

// A malformed value is an error rather than a silent fallback. Falling back
// would give an operator the default while their file says otherwise.
func TestDurationOrDefaultRefusesAMalformedValue(t *testing.T) {
	t.Parallel()
	const fallback = 90 * time.Second
	for _, value := range []string{"90", "ninety", "90 s", "s90", "1.5.2s", "90sec"} {
		got, err := durationOrDefault(value, fallback)
		if err == nil {
			t.Errorf("%q was accepted as %s", value, got)
			continue
		}
		if got == fallback {
			t.Errorf("%q returned the fallback alongside its error", value)
		}
		if !strings.Contains(err.Error(), "Go duration") {
			t.Errorf("%q: %v, expected the message to name the format", value, err)
		}
	}
}

// Zero and negative are refused rather than accepted, because a zero deadline
// is not a long one and a negative one expires before it starts.
func TestDurationOrDefaultRefusesNonPositive(t *testing.T) {
	t.Parallel()
	const fallback = 90 * time.Second
	for _, value := range []string{"0", "0s", "0ms", "-1s", "-90s"} {
		got, err := durationOrDefault(value, fallback)
		if err == nil {
			t.Errorf("%q was accepted as %s", value, got)
			continue
		}
		if !strings.Contains(err.Error(), "greater than zero") {
			t.Errorf("%q: %v, expected the message to name the bound", value, err)
		}
	}
}

// A valid value overrides the default, including the units an operator is most
// likely to reach for.
func TestDurationOrDefaultAcceptsValidUnits(t *testing.T) {
	t.Parallel()
	const fallback = 90 * time.Second
	for value, want := range map[string]time.Duration{
		"1ms":     time.Millisecond,
		"30s":     30 * time.Second,
		"5m":      5 * time.Minute,
		"1h":      time.Hour,
		"2m30s":   2*time.Minute + 30*time.Second,
		"  45s  ": 45 * time.Second,
	} {
		got, err := durationOrDefault(value, fallback)
		if err != nil {
			t.Errorf("%q: %v", value, err)
			continue
		}
		if got != want {
			t.Errorf("%q: %s, want %s", value, got, want)
		}
	}
}
