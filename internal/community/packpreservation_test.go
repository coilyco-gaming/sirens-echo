package community

import (
	"os"
	"path/filepath"
	"sort"
	"strings"
	"testing"
)

// A dataset names the pack that produced it. Twelve named a file under /tmp,
// which is scratch, so the numbers could not be re-derived. See issue 313.

// preservedPacks holds out-of-repo packs beside the agent that ran them. Kept
// out of packs/, which is the canonical set policy-check covers. See #816.
const preservedPacks = "probes"

// A cited pack that lives nowhere in the repository makes its dataset a number
// with no method, so a new one has to arrive with its pack.
func TestEveryCitedPackIsPreserved(t *testing.T) {
	t.Parallel()
	datasets, err := filepath.Glob(
		filepath.Join("..", "..", "agents", "*", "evaluations", "*.yaml"),
	)
	if err != nil || len(datasets) == 0 {
		t.Fatalf("glob datasets: %v, found %d", err, len(datasets))
	}
	missing := make([]string, 0)
	checked := 0
	for _, path := range datasets {
		pack := citedPack(t, path)
		if pack == "" || !strings.HasPrefix(pack, "/") {
			continue
		}
		checked++
		// The agent directory holding the dataset, so the pack is looked for
		// beside the run that cited it.
		agentDir := filepath.Dir(filepath.Dir(path))
		preserved := filepath.Join(agentDir, preservedPacks, filepath.Base(pack))
		if _, err := os.Stat(preserved); err != nil {
			missing = append(missing, filepath.Base(path)+" cites "+pack)
		}
	}
	if checked == 0 {
		t.Fatal("no dataset cites an out-of-repo pack, so this test stopped covering anything")
	}
	sort.Strings(missing)
	if len(missing) > 0 {
		t.Errorf("a dataset cites a pack that exists nowhere in the repository, so its "+
			"numbers cannot be re-derived. Copy the pack to that agent's %s/ and keep the "+
			"dataset's original path as the record:\n%s",
			preservedPacks, strings.Join(missing, "\n"))
	}
}

// citedPack reads provenance.pack without a YAML parse, because the datasets
// written before issue 313 carry log lines and do not unmarshal.
func citedPack(t *testing.T, path string) string {
	t.Helper()
	body, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read %s: %v", path, err)
	}
	for _, line := range strings.Split(string(body), "\n") {
		trimmed := strings.TrimSpace(line)
		if rest, found := strings.CutPrefix(trimmed, "pack:"); found {
			return strings.Trim(strings.TrimSpace(rest), `"'`)
		}
	}
	return ""
}
