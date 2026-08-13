package community

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/modelcontextprotocol/go-sdk/mcp"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	"go.opentelemetry.io/otel/sdk/trace/tracetest"
)

// bd31e36 stated the cache hit rather than leaving it to be timed, and the
// tests that shipped with it cover needsTools. See sirens-echo#520.

// listingSpanAttributes opens the provider inside a recorded mcp.tools.list
// span and returns what that span was told.
func listingSpanAttributes(
	t *testing.T,
	provider *MCPProvider,
) map[string]string {
	t.Helper()
	recorder := tracetest.NewSpanRecorder()
	traceProvider := sdktrace.NewTracerProvider(sdktrace.WithSpanProcessor(recorder))
	t.Cleanup(func() { _ = traceProvider.Shutdown(context.Background()) })
	ctx, span := traceProvider.Tracer("sirens-echo-test").Start(
		context.Background(),
		"mcp.tools.list",
	)
	session, err := provider.Open(ctx)
	if err != nil {
		span.End()
		t.Fatalf("Open: %v", err)
	}
	if err := session.Close(); err != nil {
		span.End()
		t.Fatalf("Close: %v", err)
	}
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

// liveToolServer is a reachable MCP server, so a first listing is a real one.
func liveToolServer(t *testing.T) string {
	t.Helper()
	server := mcp.NewServer(&mcp.Implementation{Name: "eco-test", Version: "1"}, nil)
	httpServer := httptest.NewServer(mcp.NewStreamableHTTPHandler(
		func(*http.Request) *mcp.Server { return server },
		&mcp.StreamableHTTPOptions{JSONResponse: true},
	))
	t.Cleanup(httpServer.Close)
	return httpServer.URL
}

// Both directions on one provider, because the attribute is only worth
// anything if the two cases it separates actually report differently.
func TestTheListingAttributeSeparatesAListingFromAHit(t *testing.T) {
	t.Parallel()
	provider := &MCPProvider{
		Servers:    []MCPServerDefinition{{Name: "eco", URL: liveToolServer(t)}},
		HTTPClient: &http.Client{Timeout: 5 * time.Second},
	}
	t.Cleanup(func() { _ = provider.Close() })

	first := listingSpanAttributes(t, provider)
	if first["mcp.tools.cached"] != "false" || first["mcp.tools.listed"] != "1" {
		t.Errorf("a first listing was not reported as one: %v", first)
	}

	// The same provider again, inside the refresh interval. Nothing changed, so
	// the roster is served from memory and the span must say so.
	second := listingSpanAttributes(t, provider)
	if second["mcp.tools.cached"] != "true" || second["mcp.tools.listed"] != "0" {
		t.Errorf("a cache hit was not reported as one: %v", second)
	}
}
