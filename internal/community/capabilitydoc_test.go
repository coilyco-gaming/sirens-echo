package community

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// capability.md tells the model what the service can do, and the model repeats
// it to members. A number that drifts from the code becomes a false claim.

const capabilityDocPath = "../../.agents/skills/sirens-echo-knowledge/references/capability.md"

func readCapabilityDoc(t *testing.T) string {
	t.Helper()
	body, err := os.ReadFile(capabilityDocPath)
	if err != nil {
		t.Fatalf("read %s: %v", capabilityDocPath, err)
	}
	return string(body)
}

// The tool-round ceiling is the stated limit on how complex a request can be,
// so the doc has to name the number the proxy actually enforces.
func TestCapabilityDocStatesTheRealToolRoundCeiling(t *testing.T) {
	t.Parallel()
	doc := readCapabilityDoc(t)

	words := map[int]string{5: "five", 6: "six", 7: "seven", 8: "eight"}
	stated, known := words[maxToolRounds]
	if !known {
		t.Fatalf("maxToolRounds = %d has no spelled form in this test; extend the map", maxToolRounds)
	}
	if !strings.Contains(doc, stated+" tool rounds") {
		t.Errorf("capability.md does not say %q; maxToolRounds is %d and the doc must match",
			stated+" tool rounds", maxToolRounds)
	}

	// The round after the ceiling is the one that fails, so the doc's off-by-one
	// has to track it. Matched as a phrase, since a number word hides in prose.
	ordinals := map[int]string{6: "sixth", 7: "seventh", 8: "eighth", 9: "ninth"}
	if next, ok := ordinals[maxToolRounds+1]; ok && !strings.Contains(doc, "on the "+next) {
		t.Errorf("capability.md does not name %q as the failing round", "on the "+next)
	}
}

// The reply cap is asserted through ParseReply rather than a constant, because
// the limit is a literal there and behavior is what the member meets.
func TestCapabilityDocStatesTheRealReplyCap(t *testing.T) {
	t.Parallel()
	doc := readCapabilityDoc(t)

	const stated = 1800
	if !strings.Contains(doc, "1800") {
		t.Errorf("capability.md does not name the %d character reply cap", stated)
	}
	if _, err := ParseReply(strings.Repeat("a", stated)); err != nil {
		t.Errorf("a reply of exactly %d runes was rejected: %v", stated, err)
	}
	if _, err := ParseReply(strings.Repeat("a", stated+1)); err == nil {
		t.Errorf("a reply of %d runes was accepted, so the doc's cap is wrong", stated+1)
	}
}

// Every lane's context window has to match the one number the doc gives the
// model, or the doc is right for one deployment and wrong for the other.
func TestCapabilityDocStatesTheRealContextWindow(t *testing.T) {
	t.Parallel()
	doc := readCapabilityDoc(t)

	words := map[string]string{"10": "ten", "11": "eleven", "12": "twelve", "13": "thirteen"}
	definitions, err := filepath.Glob("../../agent/*.yaml")
	if err != nil || len(definitions) == 0 {
		t.Fatalf("glob agent definitions: %v, found %d", err, len(definitions))
	}

	checked := 0
	for _, path := range definitions {
		body, err := os.ReadFile(path)
		if err != nil {
			t.Fatalf("read %s: %v", path, err)
		}
		value := yamlScalar(string(body), "max_context_messages")
		if value == "" {
			continue
		}
		checked++
		stated, known := words[value]
		if !known {
			t.Errorf("%s sets max_context_messages: %s, which this test cannot spell; extend the map",
				filepath.Base(path), value)
			continue
		}
		// Matched as a phrase, because a bare number word is a substring of
		// ordinary prose — "ten" hides inside "softening".
		phrase := stated + " recent channel messages"
		if !strings.Contains(doc, phrase) {
			t.Errorf("%s sets max_context_messages: %s but capability.md does not say %q",
				filepath.Base(path), value, phrase)
		}
	}
	if checked == 0 {
		t.Fatal("no agent definition declared max_context_messages")
	}
}

// The no-background-work claim holds only while the harness declares no job
// kind the deployment can enable, so the claim is tied to that closed set.
func TestCapabilityDocDoesNotDenyWorkTheHarnessCanRun(t *testing.T) {
	t.Parallel()
	doc := readCapabilityDoc(t)

	if !strings.Contains(doc, "Nothing runs between requests") {
		t.Skip("capability.md no longer makes the no-background-work claim")
	}
	for kind := range JobKinds {
		if kind == "echo" || kind == "ward-exec" {
			continue
		}
		t.Errorf("JobKinds declares %q, which is work that outlives a reply; "+
			"capability.md still tells the model nothing runs between requests", kind)
	}
}

// yamlScalar reads one top-level scalar without a YAML dependency, which the
// test does not otherwise need.
func yamlScalar(body, key string) string {
	for _, line := range strings.Split(body, "\n") {
		if !strings.HasPrefix(line, key+":") {
			continue
		}
		return strings.TrimSpace(strings.TrimPrefix(line, key+":"))
	}
	return ""
}
