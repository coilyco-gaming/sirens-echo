package community

import (
	"context"
	"net/http"
	"testing"
	"time"

	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	"go.opentelemetry.io/otel/sdk/trace/tracetest"
)

// An outage is not a cache hit. The attribute has to be right on the one turn
// it matters. See sirens-echo#540, and sirens-echo#534 for the roster size.

// discoveryAttributes runs a real Open against the given roster and returns
// what the discovery span carries. Through Open, not the counter.
func discoveryAttributes(
	t *testing.T, servers []MCPServerDefinition,
) map[string]string {
	t.Helper()
	recorder := tracetest.NewSpanRecorder()
	provider := sdktrace.NewTracerProvider(sdktrace.WithSpanProcessor(recorder))
	t.Cleanup(func() { _ = provider.Shutdown(context.Background()) })

	ctx, span := provider.Tracer("probe").Start(context.Background(), "mcp.tools.list")
	roster := &MCPProvider{
		Servers:    servers,
		HTTPClient: &http.Client{Timeout: time.Second},
	}
	_, _ = roster.Open(ctx)
	span.End()

	attributes := make(map[string]string)
	for _, ended := range recorder.Ended() {
		if ended.Name() != "mcp.tools.list" {
			continue
		}
		for _, attribute := range ended.Attributes() {
			attributes[string(attribute.Key)] = attribute.Value.Emit()
		}
	}
	return attributes
}

// The reported defect. A server that cannot be reached is a round trip that
// failed, and reporting it as cached asserts the comfortable answer.
func TestAnUnreachableServerIsNotACacheHit(t *testing.T) {
	t.Parallel()
	// A port nothing listens on, so the connect fails rather than hangs.
	got := discoveryAttributes(t, []MCPServerDefinition{
		{Name: "eco", URL: "http://127.0.0.1:1/mcp"},
	})

	if got["mcp.tools.cached"] != "false" {
		t.Errorf("an outage reported mcp.tools.cached=%q, want false",
			got["mcp.tools.cached"])
	}
	if got["mcp.tools.reached"] != "1" {
		t.Errorf("mcp.tools.reached=%q, want 1 for a connect that went out and failed",
			got["mcp.tools.reached"])
	}
	if got["mcp.tools.listed"] != "0" {
		t.Errorf("mcp.tools.listed=%q, want 0 because nothing listed",
			got["mcp.tools.listed"])
	}
	if got["mcp.tools.configured"] != "1" {
		t.Errorf("mcp.tools.configured=%q, want the roster size", got["mcp.tools.configured"])
	}
}

// The roster size is what makes the cached count derivable in the mixed case,
// without changing the type of an attribute that already shipped.
func TestTheRosterSizeIsReported(t *testing.T) {
	t.Parallel()
	got := discoveryAttributes(t, []MCPServerDefinition{
		{Name: "eco", URL: "http://127.0.0.1:1/mcp"},
		{Name: "forgejo", URL: "http://127.0.0.1:1/mcp"},
	})
	if got["mcp.tools.configured"] != "2" {
		t.Errorf("mcp.tools.configured=%q, want 2", got["mcp.tools.configured"])
	}
}

// A profile with no tools has nothing to cache, so reporting a cache hit would
// be a claim about a thing that does not exist.
func TestAnEmptyRosterIsNotCached(t *testing.T) {
	t.Parallel()
	got := discoveryAttributes(t, nil)
	if got["mcp.tools.cached"] != "false" {
		t.Errorf("an empty roster reported mcp.tools.cached=%q, want false",
			got["mcp.tools.cached"])
	}
	if got["mcp.tools.configured"] != "0" {
		t.Errorf("mcp.tools.configured=%q, want 0", got["mcp.tools.configured"])
	}
}
