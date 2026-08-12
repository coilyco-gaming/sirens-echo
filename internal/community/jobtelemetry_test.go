package community

import (
	"bytes"
	"context"
	"encoding/json"
	"log/slog"
	"strings"
	"testing"

	metricnoop "go.opentelemetry.io/otel/metric/noop"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	"go.opentelemetry.io/otel/sdk/trace/tracetest"
)

func jobTelemetry(t *testing.T) (*Telemetry, *tracetest.SpanRecorder, *bytes.Buffer) {
	t.Helper()
	var logs bytes.Buffer
	recorder := tracetest.NewSpanRecorder()
	provider := sdktrace.NewTracerProvider(sdktrace.WithSpanProcessor(recorder))
	t.Cleanup(func() { _ = provider.Shutdown(context.Background()) })
	telemetry, err := newTelemetry(
		slog.New(slog.NewJSONHandler(&logs, nil)),
		provider,
		metricnoop.NewMeterProvider(),
	)
	if err != nil {
		t.Fatalf("newTelemetry: %v", err)
	}
	return telemetry, recorder, &logs
}

// A job's whole trace is retrievable by job id, which means every span under
// it carries the id and not only the root. See #146.
func TestEverySpanUnderAJobCarriesTheJobID(t *testing.T) {
	t.Parallel()
	telemetry, recorder, _ := jobTelemetry(t)
	job := Job{ID: "job-abcdef0123", Kind: "echo", Origin: JobOrigin{Transport: transportDiscord}}

	ctx, root := telemetry.StartJobSpan(context.Background(), job)
	_, child := telemetry.StartSpan(ctx, "model.chat")
	child.End()
	_, grandchild := telemetry.StartSpan(ctx, "mcp.tool.call")
	grandchild.End()
	root.End()

	spans := recorder.Ended()
	if len(spans) != 3 {
		t.Fatalf("recorded %d spans, want 3", len(spans))
	}
	for _, span := range spans {
		var found string
		for _, attribute := range span.Attributes() {
			if string(attribute.Key) == "sirens_echo.job.id" {
				found = attribute.Value.AsString()
			}
		}
		if found != job.ID {
			t.Errorf("span %s carries job id %q, want %s", span.Name(), found, job.ID)
		}
	}
}

// A span outside a job must not gain the attribute, or the id stops meaning
// anything when it is present.
func TestSpansOutsideAJobCarryNoJobID(t *testing.T) {
	t.Parallel()
	telemetry, recorder, _ := jobTelemetry(t)
	_, span := telemetry.StartSpan(context.Background(), "community.turn")
	span.End()

	for _, attribute := range recorder.Ended()[0].Attributes() {
		if string(attribute.Key) == "sirens_echo.job.id" {
			t.Errorf("a turn span carries a job id: %v", attribute.Value.AsString())
		}
	}
}

// The log row joins trace_id and span_id, so a job's log history is
// retrievable by id without knowing the service or namespace.
func TestLogsUnderAJobCarryTheJobIDAsARowField(t *testing.T) {
	t.Parallel()
	telemetry, _, logs := jobTelemetry(t)
	job := Job{ID: "job-abcdef0123", Kind: "echo", Origin: JobOrigin{Transport: transportDiscord}}

	ctx, span := telemetry.StartJobSpan(context.Background(), job)
	telemetry.Info(ctx, "job.step")
	span.End()
	telemetry.Info(context.Background(), "turn.step")

	rows := strings.Split(strings.TrimSpace(logs.String()), "\n")
	if len(rows) != 2 {
		t.Fatalf("logged %d rows, want 2: %s", len(rows), logs.String())
	}
	var inside map[string]any
	if err := json.Unmarshal([]byte(rows[0]), &inside); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if inside["job_id"] != job.ID {
		t.Errorf("job row job_id = %v", inside["job_id"])
	}
	if inside["trace_id"] == nil || inside["span_id"] == nil {
		t.Errorf("job row lost its trace correlation: %v", inside)
	}
	var outside map[string]any
	if err := json.Unmarshal([]byte(rows[1]), &outside); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if _, present := outside["job_id"]; present {
		t.Errorf("a non-job row carries a job id: %v", outside)
	}
}

// Progress edits one line rather than posting a column of them.
func TestProgressEditsOneMessagePerJob(t *testing.T) {
	t.Parallel()
	reporter := newDiscordJobReporter(nil)
	reporter.progressMessage["job-abcdef0123"] = "message-1"

	if got := reporter.takeProgressMessage("job-abcdef0123"); got != "message-1" {
		t.Errorf("took %q", got)
	}
	if got := reporter.takeProgressMessage("job-abcdef0123"); got != "" {
		t.Errorf("a taken progress line came back: %q", got)
	}
}

// A bound thread keeps job chatter out of the channel it started in.
func TestJobRepliesPreferABoundThread(t *testing.T) {
	t.Parallel()
	reporter := newDiscordJobReporter(nil)
	job := Job{Origin: JobOrigin{ChannelID: "channel-1"}}
	if got := reporter.channelFor(job); got != "channel-1" {
		t.Errorf("channel = %q", got)
	}
	job.Origin.ThreadID = "thread-1"
	if got := reporter.channelFor(job); got != "thread-1" {
		t.Errorf("with a thread bound, channel = %q", got)
	}
}
