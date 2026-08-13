package community

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// The guard on issue 423 fires when a dataset is committed. This one fires at
// run time, which is the only moment the out-of-repo pack still exists.

func TestTheRunnerPreservesAPackItRanFromOutsideTheRepo(t *testing.T) {
	t.Parallel()
	source, err := os.ReadFile(filepath.Join("..", "..", "cmd", "sirens-echo-eval", "main.go"))
	if err != nil {
		t.Fatalf("read the runner: %v", err)
	}
	body := string(source)
	for _, want := range []struct{ fragment, why string }{
		{"preserveOutOfRepoPack(packPath)", "the runner no longer preserves an out-of-repo pack, so a dataset committed from a scratch run cannot be re-derived"},
		{"evaluations\", \"packs\"", "the copy no longer lands where the preservation guard resolves it"},
	} {
		if !strings.Contains(body, want.fragment) {
			t.Error(want.why)
		}
	}
}
