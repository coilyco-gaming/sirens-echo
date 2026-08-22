package community

import (
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
)

// runtimeStage is everything after the last FROM, which is what the published
// image actually contains.
func runtimeStage(t *testing.T) string {
	t.Helper()
	raw, err := os.ReadFile(filepath.Join("..", "..", "Dockerfile"))
	if err != nil {
		t.Fatalf("read Dockerfile: %v", err)
	}
	body := string(raw)
	last := strings.LastIndex(body, "\nFROM ")
	if last < 0 {
		t.Fatal("the Dockerfile has no stages")
	}
	return body[last:]
}

// definitionPaths is every lane definition in the tree, which is the list the
// image has to keep up with as lanes are added.
func definitionPaths(t *testing.T) []string {
	t.Helper()
	found, err := filepath.Glob(filepath.Join("..", "..", "agents", "*", "definition.yaml"))
	if err != nil {
		t.Fatalf("glob definitions: %v", err)
	}
	if len(found) == 0 {
		t.Fatal("no definition found, so this test would pass on an empty tree")
	}
	return found
}

// The working directory of a process whose whole job is answering prompts held
// the answers to its own tests. See sirens-echo#1012.
func TestTheImageShipsEveryDefinitionAndNoEvalMaterial(t *testing.T) {
	t.Parallel()
	stage := runtimeStage(t)

	for _, path := range definitionPaths(t) {
		lane := filepath.Base(filepath.Dir(path))
		want := "agents/" + lane + "/definition.yaml"
		if !strings.Contains(stage, want) {
			t.Errorf(
				"the runtime stage never copies %s, so an unset SIRENS_ECHO_DEFINITION "+
					"cannot resolve that lane",
				want,
			)
		}
	}

	// A whole-tree copy is what put the board in cwd, so the shape is refused
	// rather than the directory names being blocklisted one at a time.
	whole := regexp.MustCompile(`(?m)^COPY[^\n]*\sagents(\s|/\*)`)
	if match := whole.FindString(stage); match != "" {
		t.Errorf(
			"the runtime stage copies the agents tree wholesale: %q. "+
				"That ships probes, board cases, and graded replies into cwd",
			strings.TrimSpace(match),
		)
	}
}

// The fallback definition has to be one the image actually carries, or an
// unset environment fails on a path that shipped yesterday.
func TestTheFallbackDefinitionReachesTheImage(t *testing.T) {
	t.Parallel()
	if !strings.Contains(runtimeStage(t), defaultDefinitionPath) {
		t.Errorf(
			"defaultDefinitionPath is %q and the runtime stage does not copy it",
			defaultDefinitionPath,
		)
	}
}
