package main_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// The gate verifies a hand-written list of paths. A pack absent from that list
// ships unverified, and the build says nothing because it was never asked.

// agentDefinitionGlob is every file the gate could be asked to verify. Deriving
// the target set is the point: a list checked against a list drifts together.
const agentDefinitionGlob = "../../agent/*.yaml"

// Every pack under agent/ has to be named in main.go. A new one is unverified
// until someone remembers, and forgetting produces a green build.
func TestTheGateNamesEveryPackUnderAgent(t *testing.T) {
	t.Parallel()
	source := gateSource(t)
	packs, err := filepath.Glob(agentDefinitionGlob)
	if err != nil {
		t.Fatalf("glob %s: %v", agentDefinitionGlob, err)
	}
	if len(packs) == 0 {
		t.Fatalf("glob %s matched nothing, so this test asserts nothing", agentDefinitionGlob)
	}
	for _, pack := range packs {
		// The literal as main.go spells it, relative to the repository root.
		reference := "agent/" + filepath.Base(pack)
		if !strings.Contains(source, `"`+reference+`"`) {
			t.Errorf("%s is not verified by the gate; add it to the matching list in main.go",
				reference)
		}
	}
}

// The reverse direction. A path the gate names but nothing provides fails the
// build at run time, which is loud, but naming the stale entry here is cheaper.
func TestTheGateNamesNoPackThatIsGone(t *testing.T) {
	t.Parallel()
	for _, reference := range quotedAgentPaths(gateSource(t)) {
		if _, err := os.Stat(filepath.Join("..", "..", reference)); err != nil {
			t.Errorf("the gate verifies %s, which is not on disk: %v", reference, err)
		}
	}
}

// gateSource reads the binary's own source, because the list is a literal in it
// rather than something the package exports.
func gateSource(t *testing.T) string {
	t.Helper()
	raw, err := os.ReadFile("main.go")
	if err != nil {
		t.Fatalf("read main.go: %v", err)
	}
	return string(raw)
}

// quotedAgentPaths pulls every "agent/..." literal out of the source.
func quotedAgentPaths(source string) []string {
	found := []string{}
	for _, chunk := range strings.Split(source, `"agent/`)[1:] {
		end := strings.Index(chunk, `"`)
		if end < 0 {
			continue
		}
		found = append(found, "agent/"+chunk[:end])
	}
	return found
}
