package community

import (
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"testing"
)

// The consolidation had nothing holding it, and three constants drifted back
// out before anyone noticed. See sirens-echo#829.

// tunableName matches the names docs/sirens-echo-tuning.md calls knobs: a
// timeout, a cap, a bound, a retry count, a size limit.
var tunableName = regexp.MustCompile(
	`^(max|min|default)[A-Z]|` +
		`(Timeout|Bytes|Limit|Rounds|Retries|Backoff|Grace|Interval|Cadence)$`,
)

// numericDeclaration matches `name = 6`, `name = 8 * 1024`, `name = 10 *
// time.Second`, inside a block or as a top-level const or var.
var numericDeclaration = regexp.MustCompile(
	`^\s*(?:const\s+|var\s+)?([a-z][A-Za-z0-9_]*)\s*=\s*[0-9]+(\s*\*\s*[0-9A-Za-z_.]+)*\s*(//.*)?$`,
)

// elsewhereByDesign are numbers that read as knobs and are not. Each needs a
// reason, because the cheap way to pass this test is to add a line here.
var elsewhereByDesign = map[string]string{
	"minNormalizedIDDigits": "an algorithm's collision floor, not a knob: lowering it changes what counts as an identifier",
	"minEncodedGuardBytes":  "the same floor for the base64 reading: it decides what is a match, not how much of one to allow",
}

// A knob outside config.go is invisible to anyone asking what this service can
// be tuned to do, which is the whole reason one file holds them.
func TestEveryTuningNumberLivesInConfigGo(t *testing.T) {
	t.Parallel()
	entries, err := os.ReadDir(".")
	if err != nil {
		t.Fatalf("read package dir: %v", err)
	}
	strays := make([]string, 0)
	claimed := make(map[string]bool, len(elsewhereByDesign))
	for _, entry := range entries {
		name := entry.Name()
		if entry.IsDir() || !strings.HasSuffix(name, ".go") ||
			strings.HasSuffix(name, "_test.go") || name == "config.go" {
			continue
		}
		body, err := os.ReadFile(filepath.Join(".", name))
		if err != nil {
			t.Fatalf("read %s: %v", name, err)
		}
		for number, line := range strings.Split(string(body), "\n") {
			match := numericDeclaration.FindStringSubmatch(line)
			if match == nil || !tunableName.MatchString(match[1]) {
				continue
			}
			if _, ok := elsewhereByDesign[match[1]]; ok {
				claimed[match[1]] = true
				continue
			}
			strays = append(strays, name+":"+itoa(number+1)+" "+match[1])
		}
	}
	sort.Strings(strays)
	if len(strays) > 0 {
		t.Errorf("a tuning number lives outside config.go, so nothing lists it as a "+
			"knob. Move it there, or add it to elsewhereByDesign with a reason:\n%s",
			strings.Join(strays, "\n"))
	}
	for name, reason := range elsewhereByDesign {
		if !claimed[name] {
			t.Errorf("elsewhereByDesign names %s, which no longer exists. It was "+
				"exempt because: %s", name, reason)
		}
	}
}

// Every number goes through the one helper, so a number declared in config.go
// and left out of the table is settable by nobody.
func TestEveryNumberInConfigGoIsInTheTable(t *testing.T) {
	t.Parallel()
	body, err := os.ReadFile("config.go")
	if err != nil {
		t.Fatalf("read config.go: %v", err)
	}
	declared := make(map[string]bool)
	for _, line := range strings.Split(string(body), "\n") {
		match := numericDeclaration.FindStringSubmatch(line)
		if match == nil || !tunableName.MatchString(match[1]) {
			continue
		}
		declared[match[1]] = true
	}
	// Every knob-shaped number in the file is declared with a type and set by
	// the table, so a plain `name = 6` here is one that escaped the helper.
	for name := range declared {
		t.Errorf("%s is assigned a literal in config.go rather than declared "+
			"through overridable(), so no environment name reaches it", name)
	}
}

// Two knobs on one name means the second silently wins and the first is set by
// nothing, which reads exactly like a working override.
func TestNoTwoKnobsShareAnEnvironmentName(t *testing.T) {
	t.Parallel()
	seen := make(map[string]bool)
	for _, entry := range knobs() {
		if seen[entry.env] {
			t.Errorf("%s is declared twice, so one of the two is set by nothing", entry.env)
		}
		seen[entry.env] = true
	}
}

// Every name is this service's own, so a knob cannot collide with an unrelated
// variable a deployment already sets.
func TestEveryKnobNameIsPrefixed(t *testing.T) {
	t.Parallel()
	for _, entry := range knobs() {
		if !strings.HasPrefix(entry.env, "SIRENS_ECHO_") {
			t.Errorf("%s is not prefixed, so it can collide with something else "+
				"in the pod's environment", entry.env)
		}
	}
}

// A declared default must be the value the variable actually starts on, or the
// line reads as documentation of something that is not true.
func TestEveryKnobStartsOnItsDeclaredDefault(t *testing.T) {
	applyKnobs(func(string) string { return "" })
	for _, entry := range knobs() {
		if got := entry.value(); got != entry.fallback {
			t.Errorf("%s declares %s and its variable holds %s",
				entry.env, entry.fallback, got)
		}
	}
}

// Generated from the table, so it cannot fall behind what the code offers.
// Derive an inventory from its owner rather than keeping a second copy.
func TestTheKnobReferenceIsCurrent(t *testing.T) {
	t.Parallel()
	tracked, err := os.ReadFile(filepath.Join("..", "..", "docs", "sirens-echo-knobs.md"))
	if err != nil {
		t.Fatalf("read the reference: %v", err)
	}
	if string(tracked) != RenderKnobReference() {
		t.Error("docs/sirens-echo-knobs.md is stale. Run `ward exec knobs`.")
	}
}

// A deployment reads the prose page first, so it has to reach the list from
// there rather than from the source.
func TestTheOverrideDocPointsAtTheReference(t *testing.T) {
	t.Parallel()
	body, err := os.ReadFile(filepath.Join("..", "..", "docs", "sirens-echo-tuning-overrides.md"))
	if err != nil {
		t.Fatalf("read the overrides doc: %v", err)
	}
	if !strings.Contains(string(body), "sirens-echo-knobs.md") {
		t.Error("the overrides doc never links the generated reference, so a " +
			"deployment cannot find the list of names")
	}
}

// itoa keeps the line number readable without pulling strconv in for one call.
func itoa(value int) string {
	if value == 0 {
		return "0"
	}
	digits := ""
	for value > 0 {
		digits = string(rune('0'+value%10)) + digits
		value /= 10
	}
	return digits
}
