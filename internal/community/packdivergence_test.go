package community

import (
	"fmt"
	"path/filepath"
	"sort"
	"strings"
	"testing"

	"gopkg.in/yaml.v3"
)

// Two datasets are only comparable when the case they share is defined the same
// way in both. Three readings went wrong on this today. See issue 316.

// declaredDivergence records a case whose definition deliberately differs
// between preserved packs, with the reason a reader needs to not compare them.
var declaredDivergence = map[string]string{
	"injection-prompt-extraction": "security.yaml carries the repository-slug pattern " +
		"that bundle-security.yaml dropped, so their failure counts count different checks",
	"self-description-invents-no-path": "rate-selfdesc.yaml omits forbid_tool_call_markup, " +
		"which selfdesc-case.yaml and bundle-brevity.yaml both set",
	"no-emotional-acknowledgment": "echo-lane-check.yaml and rate-echo-original.yaml were " +
		"authored for different probes and have never been reconciled",
	"sensitive-block-nsfw": "echo-lane-check.yaml and rate-sensitive.yaml were authored " +
		"for different probes and have never been reconciled",
	"injection-fake-system-turn": "forged-marked.yaml sets asserted_history and " +
		"forged-plain.yaml does not, which is the variable 408fa6a measured",
}

// A case defined two ways in two packs makes two datasets look comparable when
// they are not, which is how a check change gets read as a model change.
func TestACaseDefinedTwiceIsIdenticalOrDeclared(t *testing.T) {
	t.Parallel()
	byCase := casesByPack(t)
	undeclared := make([]string, 0)
	declaredButIdentical := make([]string, 0)
	for id, packs := range byCase {
		if len(packs) < 2 {
			continue
		}
		variants := make(map[string][]string)
		for pack, body := range packs {
			variants[body] = append(variants[body], pack)
		}
		_, declared := declaredDivergence[id]
		if len(variants) == 1 {
			if declared {
				declaredButIdentical = append(declaredButIdentical, id)
			}
			continue
		}
		if declared {
			continue
		}
		groups := make([]string, 0, len(variants))
		for _, packs := range variants {
			sort.Strings(packs)
			groups = append(groups, strings.Join(packs, "+"))
		}
		sort.Strings(groups)
		undeclared = append(undeclared, fmt.Sprintf("%s: %s", id, strings.Join(groups, " vs ")))
	}
	sort.Strings(undeclared)
	if len(undeclared) > 0 {
		t.Errorf("a case is defined differently in two preserved packs and nothing says so, "+
			"so a reader comparing their datasets attributes a check change to the model. "+
			"Align the packs, or add the case to declaredDivergence with the reason:\n%s",
			strings.Join(undeclared, "\n"))
	}
	sort.Strings(declaredButIdentical)
	if len(declaredButIdentical) > 0 {
		t.Errorf("declaredDivergence records %s, whose packs now agree. Drop the entry so "+
			"the map keeps describing the packs", strings.Join(declaredButIdentical, ", "))
	}
}

// casesByPack maps a case ID to each preserved pack's definition of it. observed
// and runs are excluded, because a probe legitimately varies both.
func casesByPack(t *testing.T) map[string]map[string]string {
	t.Helper()
	packs, err := filepath.Glob(filepath.Join("..", "..", "agents", "*", "probes", "*.yaml"))
	if err != nil || len(packs) == 0 {
		t.Fatalf("glob preserved packs: %v, found %d", err, len(packs))
	}
	byCase := make(map[string]map[string]string)
	for _, path := range packs {
		pack, err := LoadRatePack(path)
		if err != nil {
			continue
		}
		for _, rateCase := range pack.Cases {
			body := comparableCase(t, rateCase)
			if byCase[rateCase.ID] == nil {
				byCase[rateCase.ID] = make(map[string]string)
			}
			byCase[rateCase.ID][filepath.Base(path)] = body
		}
	}
	return byCase
}

// comparableCase renders everything a case declares, by marshalling rather than
// by listing fields. A list would miss the next field somebody adds.
func comparableCase(t *testing.T, rateCase RateCase) string {
	t.Helper()
	// Runs and the rate ceiling say how hard a probe looked, not what it looked
	// for, and Observed is prose. Every other field changes what is measured.
	rateCase.Runs = 0
	rateCase.MaxFailureRate = 0
	rateCase.Observed = ""
	body, err := yaml.Marshal(rateCase)
	if err != nil {
		t.Fatalf("marshal case %s: %v", rateCase.ID, err)
	}
	return string(body)
}
