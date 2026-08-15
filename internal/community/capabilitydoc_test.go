package community

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// capability.md tells the model what the service can do, and the model repeats
// it to members. A number that drifts from the code becomes a false claim.

const capabilityDocGlob = "../../.agents/skills/*/references/capability.md"

// Each lane carries its own copy, so every assertion runs against all of them.
// One guarded copy and one unguarded copy is the drift this file exists to stop.
func capabilityDocs(t *testing.T) map[string]string {
	t.Helper()
	paths, err := filepath.Glob(capabilityDocGlob)
	if err != nil || len(paths) == 0 {
		t.Fatalf("glob %s: %v, found %d", capabilityDocGlob, err, len(paths))
	}
	docs := make(map[string]string, len(paths))
	for _, path := range paths {
		body, err := os.ReadFile(path)
		if err != nil {
			t.Fatalf("read %s: %v", path, err)
		}
		docs[lane(path)] = string(body)
	}
	return docs
}

// lane names a copy by its skill root, which is what a failure needs to say.
func lane(path string) string {
	return filepath.Base(filepath.Dir(filepath.Dir(path)))
}

// The outer budget is what actually fired in issue 258, at a depth past the
// tool-round ceiling, so the doc has to name it too.
func TestCapabilityDocStatesTheModelCallBudget(t *testing.T) {
	t.Parallel()
	words := map[int]string{8: "eight", 9: "nine", 10: "ten", 11: "eleven", 12: "twelve"}
	budget := maxToolRounds + maxResponseRepairs + budgetRaisesAllowed + 1
	stated, known := words[budget]
	if !known {
		t.Fatalf("the model-call budget is %d and has no spelled form here; extend the map", budget)
	}
	for name, doc := range capabilityDocs(t) {
		if !strings.Contains(doc, stated+" model calls") {
			t.Errorf("%s does not say %q; the budget is %d and the doc must match",
				name, stated+" model calls", budget)
		}
	}
}

// The tool-round ceiling is the stated limit on how complex a request can be,
// so the doc has to name the number the proxy actually enforces.
func TestCapabilityDocStatesTheRealToolRoundCeiling(t *testing.T) {
	t.Parallel()
	words := map[int]string{5: "five", 6: "six", 7: "seven", 8: "eight"}
	stated, known := words[maxToolRounds]
	if !known {
		t.Fatalf("maxToolRounds = %d has no spelled form in this test; extend the map", maxToolRounds)
	}
	// The last round is the one after which tools are withdrawn, so the doc has
	// to name it. Matched as a phrase, since a number word hides in prose.
	ordinals := map[int]string{5: "fifth", 6: "sixth", 7: "seventh", 8: "eighth"}

	for name, doc := range capabilityDocs(t) {
		if !strings.Contains(doc, stated+" tool rounds") {
			t.Errorf("%s does not say %q; maxToolRounds is %d and the doc must match",
				name, stated+" tool rounds", maxToolRounds)
		}
		if last, ok := ordinals[maxToolRounds]; ok && !strings.Contains(doc, "the "+last) {
			t.Errorf("%s does not name %q as the last round", name, "the "+last)
		}
		// The turn answers from the results rather than failing, so a doc still
		// promising a failure would send a member the wrong expectation.
		if strings.Contains(doc, "fails outright") {
			t.Errorf("%s still says the turn fails outright; it degrades to an answer", name)
		}
	}
}

// The reply cap is asserted through ParseReply rather than a constant, because
// the limit is a literal there and behavior is what the member meets.
func TestCapabilityDocStatesTheRealReplyCap(t *testing.T) {
	t.Parallel()
	const stated = 1800
	for name, doc := range capabilityDocs(t) {
		if !strings.Contains(doc, "1800") {
			t.Errorf("%s does not name the %d character reply cap", name, stated)
		}
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
	docs := capabilityDocs(t)
	words := map[string]string{"10": "ten", "11": "eleven", "12": "twelve", "13": "thirteen"}
	definitions, err := filepath.Glob("../../agents/*/definition.yaml")
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
		// A phrase, because a bare number word hides in prose — "ten" is inside
		// "softening". It stops before the noun, which differs per lane.
		phrase := stated + " recent"
		for name, doc := range docs {
			if !strings.Contains(doc, phrase) {
				t.Errorf("%s sets max_context_messages: %s but %s does not say %q",
					filepath.Base(path), value, name, phrase)
			}
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
	claiming := make([]string, 0, 2)
	for name, doc := range capabilityDocs(t) {
		if strings.Contains(doc, "Nothing runs between requests") {
			claiming = append(claiming, name)
		}
	}
	if len(claiming) == 0 {
		t.Skip("no capability.md makes the no-background-work claim")
	}
	for kind := range JobKinds {
		if kind == "echo" || kind == "ward-exec" {
			continue
		}
		t.Errorf("JobKinds declares %q, which is work that outlives a reply; %v "+
			"still tell the model nothing runs between requests", kind, claiming)
	}
}

// The limits capability.md states are properties of the harness, not of one
// agent's persona, so every lane needs them and only one lane has them.
func TestCapabilityDocReachesEveryAgent(t *testing.T) {
	t.Parallel()

	// Every agent declaring skill roots must reach a capability reference. The
	// last exemption went when sirens-deep gained one.
	without := map[string]string{}

	definitions, err := filepath.Glob("../../agents/*/definition.yaml")
	if err != nil || len(definitions) == 0 {
		t.Fatalf("glob agent definitions: %v, found %d", err, len(definitions))
	}
	checked := 0
	for _, path := range definitions {
		body, err := os.ReadFile(path)
		if err != nil {
			t.Fatalf("read %s: %v", path, err)
		}
		definition := string(body)
		if !strings.Contains(definition, "local_skill_roots:") {
			continue
		}
		checked++
		name := filepath.Base(path)
		reaches := agentReachesCapabilityDoc(t, definition)
		issue, expected := without[name]
		switch {
		case reaches && expected:
			t.Errorf("%s now reaches capability.md; issue %s is fixed, so drop it "+
				"from the without map and let this assert the invariant", name, issue)
		case !reaches && !expected:
			t.Errorf("%s declares skill roots but none carries references/capability.md, "+
				"so its model is told none of the harness limits", name)
		}
	}
	if checked == 0 {
		t.Fatal("no agent definition declared local_skill_roots")
	}
}

// agentReachesCapabilityDoc reports whether any declared root holds the
// capability reference on disk.
func agentReachesCapabilityDoc(t *testing.T, body string) bool {
	t.Helper()
	for _, line := range strings.Split(body, "\n") {
		root := strings.TrimSpace(strings.TrimPrefix(strings.TrimSpace(line), "- "))
		if !strings.HasPrefix(root, ".agents/skills/") {
			continue
		}
		if _, err := os.Stat(filepath.Join("../..", root, "references", "capability.md")); err == nil {
			return true
		}
	}
	return false
}

// The doc hands members a source link, so the address has to be the repository
// this module actually is. A move breaks the module path and this with it.
func TestCapabilityDocLinksThisRepository(t *testing.T) {
	t.Parallel()
	raw, err := os.ReadFile(filepath.Join("..", "..", "go.mod"))
	if err != nil {
		t.Fatalf("read go.mod: %v", err)
	}
	module := ""
	for _, line := range strings.Split(string(raw), "\n") {
		if after, found := strings.CutPrefix(line, "module "); found {
			module = strings.TrimSpace(after)
			break
		}
	}
	if module == "" {
		t.Fatal("go.mod declares no module path")
	}
	link := "https://" + module + "/src/branch/main/"
	for name, doc := range capabilityDocs(t) {
		if !strings.Contains(doc, link) {
			t.Errorf("%s does not give the link form %q; the module is %s",
				name, link, module)
		}
	}
}

// The doc tells the model it cannot know which revision produced a reply. That
// is only true while the build stamps nothing, so the build is the assertion.
func TestCapabilityDocIsRightThatTheBuildCarriesNoRevision(t *testing.T) {
	t.Parallel()
	claim := "built without its commit"
	claiming := make([]string, 0, 2)
	// Matched against reflowed text, because the claim wraps across lines in the
	// doc and a raw Contains would skip this whole test without saying so.
	for name, doc := range capabilityDocs(t) {
		if strings.Contains(strings.Join(strings.Fields(doc), " "), claim) {
			claiming = append(claiming, name)
		}
	}
	if len(claiming) == 0 {
		t.Skip("no capability.md claims the build carries no revision")
	}

	raw, err := os.ReadFile(filepath.Join("..", "..", "Dockerfile"))
	if err != nil {
		t.Fatalf("read Dockerfile: %v", err)
	}
	body := string(raw)
	// A .git directory lets the toolchain stamp vcs.revision on its own, and an
	// -X assignment sets a variable outright. Either one makes the claim false.
	for _, stamp := range []string{"COPY .git", "-ldflags", "-X "} {
		if strings.Contains(body, stamp) {
			t.Errorf("the build stage carries %q, so the revision is knowable; %v "+
				"still tell the model the build carries no commit", stamp, claiming)
		}
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
