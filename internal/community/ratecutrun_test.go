package community

import (
	"context"
	"strings"
	"testing"
	"time"

	"gopkg.in/yaml.v3"
)

// A wedged route produced ten minutes of attempts and zero bytes of dataset,
// because the run only wrote at the end. See sirens-echo#324.

// A client that answers instantly, so the test measures the runner rather than
// a model.
type instantCompletions struct{}

func (instantCompletions) Complete(
	_ context.Context, _ TurnPrompt, _ string,
) (CompletionResult, error) {
	return CompletionResult{Content: "The Eco server is online."}, nil
}

func cutRunPack() RatePack {
	return RatePack{
		Schema: RateSchema,
		Cases: []RateCase{
			{
				EvaluationCase: EvaluationCase{
					ID:      "first",
					Current: TranscriptEntry{Author: "m", Content: "one"},
				},
				Runs: 1,
			},
			{
				EvaluationCase: EvaluationCase{
					ID:      "second",
					Current: TranscriptEntry{Author: "m", Content: "two"},
				},
				Runs: 1,
			},
		},
	}
}

func TestRateEmitsWhatItMeasuredWhenTheRunIsCut(t *testing.T) {
	t.Parallel()
	ctx, cancel := context.WithCancel(context.Background())
	var out strings.Builder
	// Cancelled before the first case, which is the shape of an interrupt
	// arriving while a wedged route is being waited on.
	cancel()
	err := runRate(
		ctx, Definition{}, PlaceholderPrincipal, "", cutRunPack(),
		RateProvenance{}, instantCompletions{}, &out, time.Second,
	)
	if err == nil {
		t.Fatal("a cut run reported success")
	}
	if !strings.Contains(err.Error(), "cut short") {
		t.Errorf("error does not say the run was cut: %v", err)
	}
	// The dataset is the point. An error without one repeats the wait.
	if out.Len() == 0 {
		t.Fatal("a cut run wrote no dataset, which is the defect")
	}
	var dataset RateDataset
	if err := yaml.Unmarshal([]byte(out.String()), &dataset); err != nil {
		t.Fatalf("the emitted dataset does not parse: %v", err)
	}
	if dataset.Provenance.Composed == "" {
		t.Error("a cut dataset lost its provenance, so it cannot be interpreted")
	}
}

// A run that completes must be unchanged: same dataset, same verdict.
func TestRateIsUnchangedWhenTheRunCompletes(t *testing.T) {
	t.Parallel()
	var out strings.Builder
	err := runRate(
		context.Background(), Definition{}, PlaceholderPrincipal, "", cutRunPack(),
		RateProvenance{}, instantCompletions{}, &out, time.Second,
	)
	if err != nil && strings.Contains(err.Error(), "cut short") {
		t.Fatalf("a complete run reported itself cut: %v", err)
	}
	var dataset RateDataset
	if err := yaml.Unmarshal([]byte(out.String()), &dataset); err != nil {
		t.Fatalf("dataset does not parse: %v", err)
	}
	if len(dataset.Records) != 2 {
		t.Errorf("records = %d, want both cases measured", len(dataset.Records))
	}
}
