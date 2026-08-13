package main_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// policy-check names its inputs by hand, so a pack added to agent/ and not
// added to the list is verified by nothing while the output still reads green.

func TestEveryTrackedPackIsVerified(t *testing.T) {
	t.Parallel()
	source, err := os.ReadFile("main.go")
	if err != nil {
		t.Fatalf("read main.go: %v", err)
	}
	packs, err := filepath.Glob(filepath.Join("..", "..", "agent", "*.yaml"))
	if err != nil || len(packs) == 0 {
		t.Fatalf("glob agent/*.yaml: %v, found %d", err, len(packs))
	}
	for _, pack := range packs {
		reference := "agent/" + filepath.Base(pack)
		if !strings.Contains(string(source), reference) {
			t.Errorf("%s is verified by nothing. Add it to the right verify "+
				"call in main.go, or delete it if it is no longer tracked.", reference)
		}
	}
}
