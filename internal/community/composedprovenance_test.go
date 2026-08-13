package community

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"gopkg.in/yaml.v3"
)

// Every Deep run substitutes a 249 byte placeholder where a deployed pod injects
// the real bundle, so a dataset has to say so. See sirens-echo#316.

func TestComposedForRunPairsTheBundleWithItsLabel(t *testing.T) {
	t.Parallel()
	bundle, recorded := composedForRun(Definition{Composed: true})
	if bundle != PlaceholderComposed {
		t.Error("a composed definition did not receive the placeholder")
	}
	if recorded != ComposedStubbed {
		t.Errorf("a stubbed run recorded %q, so a reader cannot tell it was stubbed", recorded)
	}
	bundle, recorded = composedForRun(Definition{Composed: false})
	if bundle != "" {
		t.Error("an uncomposed definition received a bundle it does not declare")
	}
	if recorded != ComposedNotRequested {
		t.Errorf("an uncomposed run recorded %q rather than %q", recorded, ComposedNotRequested)
	}
}

// The board is graded by a human reading replies, and the same substitution
// applies, so the same statement has to travel with it.
func TestABoardDatasetRecordsTheComposedStateToo(t *testing.T) {
	t.Parallel()
	definition, skillpack := rateFixtureDefinition(t)
	definition.Composed = true
	client := &scriptedCompletionClient{reply: func(string) (CompletionResult, error) {
		return CompletionResult{Content: "A reply."}, nil
	}}
	var out strings.Builder
	err := RunBoard(
		context.Background(), definition, PlaceholderPrincipal, skillpack,
		loadBoardFixture(t),
		BoardProvenance{Epochs: 1, Composed: "the-real-bundle-v9"},
		client, &out,
	)
	if err != nil {
		t.Fatalf("run board: %v", err)
	}
	var dataset BoardDataset
	if err := yaml.Unmarshal([]byte(out.String()), &dataset); err != nil {
		t.Fatalf("unmarshal dataset: %v", err)
	}
	if dataset.Provenance.Composed != ComposedStubbed {
		t.Errorf("board dataset recorded composed %q, want %q",
			dataset.Provenance.Composed, ComposedStubbed)
	}
}

// A limitation recorded in a field nobody documents is a limitation nobody
// meets, since a reader of a dataset reads the provenance doc to interpret it.
func TestTheProvenanceDocExplainsTheStub(t *testing.T) {
	t.Parallel()
	body, err := os.ReadFile(filepath.Join("..", "..", "docs", "sirens-echo-rate-provenance.md"))
	if err != nil {
		t.Fatalf("read the provenance doc: %v", err)
	}
	flat := strings.Join(strings.Fields(string(body)), " ")
	for _, want := range []string{"`composed`", "stubbed", "agent-compose"} {
		if !strings.Contains(flat, want) {
			t.Errorf("the provenance doc does not mention %s, so a reader of a "+
				"Deep dataset has no way to learn the bundle was absent", want)
		}
	}
}
