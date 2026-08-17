package community

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"log/slog"
	"strings"
	"testing"
)

// One line reaches stdout and OTLP both, so neither destination can be the only
// record of an event. See docs/sirens-echo-rate.md and sirens-echo#810.

// recordingHandler stands in for the OTLP side, which needs a collector.
type recordingHandler struct {
	records *[]slog.Record
	attrs   []slog.Attr
	level   slog.Level
	err     error
}

func (h recordingHandler) Enabled(_ context.Context, level slog.Level) bool {
	return level >= h.level
}

func (h recordingHandler) Handle(_ context.Context, record slog.Record) error {
	for _, attr := range h.attrs {
		record.AddAttrs(attr)
	}
	*h.records = append(*h.records, record)
	return h.err
}

func (h recordingHandler) WithAttrs(attrs []slog.Attr) slog.Handler {
	h.attrs = append(append([]slog.Attr{}, h.attrs...), attrs...)
	return h
}

func (h recordingHandler) WithGroup(string) slog.Handler { return h }

// The whole point of the change: dropping stdout for OTLP would cost kubectl
// logs during an incident, so a line has to land in both places.
func TestALineReachesStdoutAndTheExporter(t *testing.T) {
	t.Parallel()
	var stdout bytes.Buffer
	var exported []slog.Record
	logger := slog.New(multiHandler{handlers: []slog.Handler{
		slog.NewJSONHandler(&stdout, &slog.HandlerOptions{Level: slog.LevelInfo}),
		recordingHandler{records: &exported, level: slog.LevelInfo},
	}})

	logger.Info("turn.completed", slog.String("request_id", "abc"))

	if len(exported) != 1 {
		t.Fatalf("the exporter saw %d records, want 1", len(exported))
	}
	if exported[0].Message != "turn.completed" {
		t.Errorf("exported message = %q", exported[0].Message)
	}
	var row map[string]any
	if err := json.Unmarshal(stdout.Bytes(), &row); err != nil {
		t.Fatalf("stdout is not one JSON row: %v, got %q", err, stdout.String())
	}
	if row["msg"] != "turn.completed" || row["request_id"] != "abc" {
		t.Errorf("stdout row = %v", row)
	}
}

// A collector that is refusing writes must not take stdout down with it, which
// is exactly the incident where kubectl logs is the remaining path.
func TestAFailingExporterStillLeavesTheLineOnStdout(t *testing.T) {
	t.Parallel()
	var stdout bytes.Buffer
	var exported []slog.Record
	handler := multiHandler{handlers: []slog.Handler{
		slog.NewJSONHandler(&stdout, &slog.HandlerOptions{Level: slog.LevelInfo}),
		recordingHandler{
			records: &exported,
			level:   slog.LevelInfo,
			err:     errors.New("collector refused the batch"),
		},
	}}
	record := slog.Record{Message: "turn.failed", Level: slog.LevelError}

	err := handler.Handle(context.Background(), record)

	if err == nil {
		t.Error("the exporter failure was swallowed rather than reported")
	}
	if !strings.Contains(stdout.String(), "turn.failed") {
		t.Errorf("stdout lost the line when the exporter failed: %q", stdout.String())
	}
}

// Attributes added to a logger must reach every destination, or the two rows
// for one event disagree about what happened.
func TestWithAttrsReachesEveryDestination(t *testing.T) {
	t.Parallel()
	var stdout bytes.Buffer
	var exported []slog.Record
	base := multiHandler{handlers: []slog.Handler{
		slog.NewJSONHandler(&stdout, &slog.HandlerOptions{Level: slog.LevelInfo}),
		recordingHandler{records: &exported, level: slog.LevelInfo},
	}}

	slog.New(base.WithAttrs([]slog.Attr{slog.String("lane", "deep")})).
		Info("turn.started")

	if !strings.Contains(stdout.String(), `"lane":"deep"`) {
		t.Errorf("stdout lost the attribute: %q", stdout.String())
	}
	if len(exported) != 1 {
		t.Fatalf("the exporter saw %d records, want 1", len(exported))
	}
	var found bool
	exported[0].Attrs(func(attr slog.Attr) bool {
		if attr.Key == "lane" && attr.Value.String() == "deep" {
			found = true
		}
		return true
	})
	if !found {
		t.Error("the exporter lost the attribute")
	}
}

// Deriving must not reshape the handler it came from, or one logger's
// attributes leak into every other logger built from the same base.
func TestDerivingDoesNotMutateTheOriginal(t *testing.T) {
	t.Parallel()
	var firstOut, secondOut bytes.Buffer
	var exported []slog.Record
	base := multiHandler{handlers: []slog.Handler{
		slog.NewJSONHandler(&firstOut, &slog.HandlerOptions{Level: slog.LevelInfo}),
		recordingHandler{records: &exported, level: slog.LevelInfo},
	}}
	_ = base.WithAttrs([]slog.Attr{slog.String("lane", "deep")})

	// The base must still write where it always did, with no borrowed attribute.
	base.handlers[0] = slog.NewJSONHandler(&secondOut, &slog.HandlerOptions{Level: slog.LevelInfo})
	slog.New(base).Info("turn.started")

	if strings.Contains(secondOut.String(), "lane") {
		t.Errorf("the derived handler's attribute leaked into the base: %q", secondOut.String())
	}
}

// A destination that wants a level keeps getting it even when another does not,
// so raising stdout's threshold cannot silence the exported copy.
func TestOneQuietDestinationDoesNotSilenceTheOther(t *testing.T) {
	t.Parallel()
	var stdout bytes.Buffer
	var exported []slog.Record
	handler := multiHandler{handlers: []slog.Handler{
		slog.NewJSONHandler(&stdout, &slog.HandlerOptions{Level: slog.LevelError}),
		recordingHandler{records: &exported, level: slog.LevelInfo},
	}}
	if !handler.Enabled(context.Background(), slog.LevelInfo) {
		t.Fatal("an info line was refused although one destination wanted it")
	}

	slog.New(handler).Info("turn.started")

	if len(exported) != 1 {
		t.Errorf("the exporter saw %d info records, want 1", len(exported))
	}
	if stdout.Len() != 0 {
		t.Errorf("stdout took a line below its own level: %q", stdout.String())
	}
}
