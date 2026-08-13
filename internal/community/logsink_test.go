package community

import (
	"bytes"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"gopkg.in/yaml.v3"
)

// The runner writes its dataset to stdout. Logs sharing that stream made every
// emitted dataset unparsable. See docs/sirens-echo-rate.md and issue 313.

func TestLogSinkDefaultsToStdoutAndHonoursAWriter(t *testing.T) {
	t.Parallel()
	if logSink(nil) != os.Stdout {
		t.Error("an unset writer must leave the service logging to stdout")
	}
	var buf bytes.Buffer
	if logSink(&buf) != io.Writer(&buf) {
		t.Error("a configured writer was ignored, so a dataset stream stays contaminated")
	}
}

// The runner writes its dataset to stdout, so it has to select stderr. A
// source check, because the alternative is running the binary.
func TestTheEvaluationRunnerKeepsLogsOffStdout(t *testing.T) {
	t.Parallel()
	body, err := os.ReadFile(filepath.Join("..", "..", "cmd", "sirens-echo-eval", "main.go"))
	if err != nil {
		t.Fatalf("read the runner: %v", err)
	}
	if !strings.Contains(string(body), "LogWriter:    os.Stderr") {
		t.Error("the evaluation runner no longer sends logs to stderr, so every " +
			"emitted dataset carries log lines and cannot be unmarshalled")
	}
}

// A dataset that cannot be unmarshalled is a weak kind of evidence, and every
// file emitted before this fix carries log lines that break the parse.
func TestADatasetStreamCarriesNoLogLines(t *testing.T) {
	t.Parallel()
	dataset := RateDataset{
		Schema: RateDatasetSchema,
		Provenance: RateProvenance{
			Definition: "agent/sirens-deep.yaml",
			Pack:       "agent/rate-deep.yaml",
		},
		Records: []RateRecord{{
			ID:       "probe",
			Runs:     1,
			Attempts: 1,
			Passed:   1,
			Measured: true,
			Responses: []RateRun{{
				Run:     1,
				Outcome: "pass",
				Text:    "a reply carrying a brace { and a quote \" to stress the encoder",
			}},
		}},
	}
	encoded, err := yaml.Marshal(dataset)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	// A log line is JSON at the start of a line, which is what broke the parse.
	for _, line := range strings.Split(string(encoded), "\n") {
		if strings.HasPrefix(line, "{\"time\"") {
			t.Fatalf("the dataset stream carries a log line: %s", line)
		}
	}
	var round RateDataset
	if err := yaml.Unmarshal(encoded, &round); err != nil {
		t.Fatalf("an emitted dataset did not unmarshal: %v", err)
	}
	if len(round.Records) != 1 || round.Records[0].ID != "probe" {
		t.Fatalf("round trip lost the record: %+v", round.Records)
	}
}
