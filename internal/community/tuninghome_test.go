package community

import (
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"testing"
)

// The consolidation onto tuning.go had nothing holding it, and three constants
// drifted back out before anyone noticed. See sirens-echo#829.

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

// A knob outside tuning.go is invisible to anyone asking what this service can
// be tuned to do, which is the whole reason the file exists.
func TestEveryTuningNumberLivesInTuningGo(t *testing.T) {
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
			strings.HasSuffix(name, "_test.go") || name == "tuning.go" {
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
		t.Errorf("a tuning number lives outside tuning.go, so nothing lists it as a "+
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

// The doc said seven while the table held eight, which is the failure mode of
// a list maintained in two places. See sirens-echo#829.
func TestTheOverrideDocNamesEveryOverridableNumber(t *testing.T) {
	t.Parallel()
	body, err := os.ReadFile(filepath.Join("..", "..", "docs", "sirens-echo-tuning-overrides.md"))
	if err != nil {
		t.Fatalf("read the overrides doc: %v", err)
	}
	doc := string(body)
	for name := range tuningOverrides() {
		if !strings.Contains(doc, name) {
			t.Errorf("%s is overridable and the doc never names it, so a deployment "+
				"cannot find it", name)
		}
	}
	for _, line := range strings.Split(doc, "\n") {
		name := strings.TrimSpace(line)
		if !strings.HasPrefix(name, "SIRENS_ECHO_") {
			continue
		}
		name = strings.Fields(name)[0]
		if _, ok := tuningOverrides()[name]; !ok {
			t.Errorf("the doc lists %s as overridable and the table has no such "+
				"entry, so setting it does nothing", name)
		}
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
