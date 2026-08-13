package community

import (
	"context"
	"io"
	"log/slog"
	"net/http"
	"testing"
	"time"

	metricnoop "go.opentelemetry.io/otel/metric/noop"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	"go.opentelemetry.io/otel/sdk/trace/tracetest"
)

// A rejection during discovery was a bare HTTP POST carrying a URL, so nobody
// could say which server or which stage. See sirens-echo#139.

// discoverySpans runs a real Open and returns the discovery spans. The provider
// is injected, so two of these running at once cannot record into each other.
func discoverySpans(t *testing.T, servers []MCPServerDefinition) []sdktrace.ReadOnlySpan {
	t.Helper()
	recorder := tracetest.NewSpanRecorder()
	provider := sdktrace.NewTracerProvider(sdktrace.WithSpanProcessor(recorder))
	t.Cleanup(func() { _ = provider.Shutdown(context.Background()) })
	telemetry, err := newTelemetry(
		slog.New(slog.NewJSONHandler(io.Discard, nil)),
		provider,
		metricnoop.NewMeterProvider(),
	)
	if err != nil {
		t.Fatalf("telemetry: %v", err)
	}

	roster := &MCPProvider{
		Servers:    servers,
		HTTPClient: &http.Client{Timeout: time.Second},
		Telemetry:  telemetry,
	}
	_, _ = roster.Open(context.Background())

	var discovery []sdktrace.ReadOnlySpan
	for _, ended := range recorder.Ended() {
		if ended.Name() == "mcp.server.discovery" {
			discovery = append(discovery, ended)
		}
	}
	return discovery
}

func spanAttribute(span sdktrace.ReadOnlySpan, key string) string {
	for _, attribute := range span.Attributes() {
		if string(attribute.Key) == key {
			return attribute.Value.Emit()
		}
	}
	return ""
}

// The reported gap. A server that rejects during discovery is now named, and so
// is the stage it was in when it failed.
func TestDiscoveryNamesTheServerAndTheStage(t *testing.T) {
	t.Parallel()
	spans := discoverySpans(t, []MCPServerDefinition{
		{Name: "steam", URL: "http://127.0.0.1:1/mcp"},
	})

	if len(spans) != 1 {
		t.Fatalf("recorded %d discovery spans, want 1. A noop provider records "+
			"none, so this test asserts nothing if it is zero", len(spans))
	}
	if got := spanAttribute(spans[0], "mcp.server.name"); got != "steam" {
		t.Errorf("mcp.server.name = %q, want the rostered name", got)
	}
	if got := spanAttribute(spans[0], "mcp.discovery.stage"); got != discoveryStageConnect {
		t.Errorf("mcp.discovery.stage = %q, want %q for a connect that failed",
			got, discoveryStageConnect)
	}
}

// One span per server, so a roster where one member fails does not report the
// failure against whichever server happened to be first.
func TestEachServerGetsItsOwnDiscoverySpan(t *testing.T) {
	t.Parallel()
	spans := discoverySpans(t, []MCPServerDefinition{
		{Name: "steam", URL: "http://127.0.0.1:1/mcp"},
		{Name: "forgejo", URL: "http://127.0.0.1:1/mcp"},
	})

	if len(spans) != 2 {
		t.Fatalf("recorded %d discovery spans, want one per server", len(spans))
	}
	named := map[string]bool{}
	for _, span := range spans {
		named[spanAttribute(span, "mcp.server.name")] = true
	}
	for _, want := range []string{"steam", "forgejo"} {
		if !named[want] {
			t.Errorf("no discovery span named %q", want)
		}
	}
}

// A roster with nothing on it does no round trips, so it records no spans.
func TestAnEmptyRosterRecordsNoDiscoverySpan(t *testing.T) {
	t.Parallel()
	if spans := discoverySpans(t, nil); len(spans) != 0 {
		t.Errorf("recorded %d discovery spans for an empty roster", len(spans))
	}
}
