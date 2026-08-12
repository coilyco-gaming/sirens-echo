package community

import (
	"bytes"
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"
	"unicode/utf8"

	"github.com/modelcontextprotocol/go-sdk/mcp"
	"go.opentelemetry.io/contrib/instrumentation/net/http/otelhttp"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/propagation"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	"go.opentelemetry.io/otel/sdk/trace/tracetest"
	"go.opentelemetry.io/otel/trace"
)

func TestMCPProviderAllowsEmptyRoster(t *testing.T) {
	t.Parallel()
	session, err := (&MCPProvider{}).Open(context.Background())
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	if tools := session.Tools(); len(tools) != 0 {
		t.Fatalf("tools = %#v", tools)
	}
	if err := session.Close(); err != nil {
		t.Fatalf("Close: %v", err)
	}
}

// stdioFixtureEnv turns this test binary into an MCP server over stdio, so the
// stdio transport is exercised against a real child process.
const stdioFixtureEnv = "SIRENS_ECHO_TEST_STDIO_MCP"

func TestMain(m *testing.M) {
	if os.Getenv(stdioFixtureEnv) != "1" {
		os.Exit(m.Run())
	}
	server := mcp.NewServer(&mcp.Implementation{Name: "stdio-fixture", Version: "1"}, nil)
	mcp.AddTool(
		server,
		&mcp.Tool{Name: "ping", Description: "fixture tool"},
		func(context.Context, *mcp.CallToolRequest, struct{}) (
			*mcp.CallToolResult, any, error,
		) {
			return &mcp.CallToolResult{
				Content: []mcp.Content{&mcp.TextContent{Text: "pong"}},
			}, nil, nil
		},
	)
	if err := server.Run(context.Background(), &mcp.StdioTransport{}); err != nil {
		os.Exit(1)
	}
}

func TestMCPProviderConnectsOverStdio(t *testing.T) {
	// Also covers env forwarding: the child only becomes a server because the
	// roster names this variable, and nothing else from Echo's environment.
	t.Setenv(stdioFixtureEnv, "1")

	session, err := (&MCPProvider{Servers: []MCPServerDefinition{{
		Name:      "local",
		Transport: MCPTransportStdio,
		Command:   os.Args[0],
		Env:       []string{stdioFixtureEnv},
	}}}).Open(context.Background())
	if err != nil {
		t.Fatalf("Open over stdio: %v", err)
	}
	defer func() { _ = session.Close() }()

	tools := session.Tools()
	if len(tools) != 1 || tools[0].Name != "local__ping" {
		t.Fatalf("tools = %#v, want the stdio server's tool", tools)
	}
	result, err := session.Call(context.Background(), "local__ping", nil)
	if err != nil {
		t.Fatalf("Call over stdio: %v", err)
	}
	if result.Text != "pong" {
		t.Fatalf("result = %q, want pong", result.Text)
	}
}

func TestMCPProviderReusesConnectionsAcrossTurns(t *testing.T) {
	t.Parallel()
	var initializes atomic.Int32
	server := mcp.NewServer(&mcp.Implementation{Name: "eco-test", Version: "1"}, nil)
	mcp.AddTool(
		server,
		&mcp.Tool{Name: "status", Description: "fixture tool"},
		func(context.Context, *mcp.CallToolRequest, struct{}) (
			*mcp.CallToolResult, any, error,
		) {
			return &mcp.CallToolResult{
				Content: []mcp.Content{&mcp.TextContent{Text: "ok"}},
			}, nil, nil
		},
	)
	handler := mcp.NewStreamableHTTPHandler(
		func(*http.Request) *mcp.Server { return server },
		&mcp.StreamableHTTPOptions{JSONResponse: true},
	)
	httpServer := httptest.NewServer(http.HandlerFunc(func(
		writer http.ResponseWriter,
		request *http.Request,
	) {
		if request.Method == http.MethodPost {
			body, _ := io.ReadAll(request.Body)
			request.Body = io.NopCloser(bytes.NewReader(body))
			if bytes.Contains(body, []byte(`"method":"initialize"`)) {
				initializes.Add(1)
			}
		}
		handler.ServeHTTP(writer, request)
	}))
	t.Cleanup(httpServer.Close)

	provider := &MCPProvider{Servers: []MCPServerDefinition{{Name: "eco", URL: httpServer.URL}}}
	t.Cleanup(func() { _ = provider.Close() })

	for turn := 0; turn < 3; turn++ {
		session, err := provider.Open(context.Background())
		if err != nil {
			t.Fatalf("turn %d Open: %v", turn, err)
		}
		if tools := session.Tools(); len(tools) != 1 {
			t.Fatalf("turn %d tools = %#v", turn, tools)
		}
		// The view borrows the supervised session, so closing it must not tear
		// the connection down.
		if err := session.Close(); err != nil {
			t.Fatalf("turn %d Close: %v", turn, err)
		}
	}
	if got := initializes.Load(); got != 1 {
		t.Fatalf("initialize requests = %d, want 1 across three turns", got)
	}
}

func TestMCPProviderRelistsWhenMarkedStale(t *testing.T) {
	t.Parallel()
	url := liveMCPServer(t, "eco-test", "status")
	provider := &MCPProvider{Servers: []MCPServerDefinition{{Name: "eco", URL: url}}}
	t.Cleanup(func() { _ = provider.Close() })

	first, err := provider.Open(context.Background())
	if err != nil {
		t.Fatalf("first Open: %v", err)
	}
	_ = first.Close()

	provider.mu.Lock()
	entry := provider.entries[0]
	provider.mu.Unlock()
	if entry.notifies {
		t.Fatal("streamable with standalone SSE disabled cannot receive notifications")
	}
	cached := entry.refreshed
	entry.stale.Store(true)

	if _, err := provider.Open(context.Background()); err != nil {
		t.Fatalf("second Open: %v", err)
	}
	provider.mu.Lock()
	refreshed := provider.entries[0].refreshed
	provider.mu.Unlock()
	if !refreshed.After(cached) {
		t.Fatal("a stale roster was not re-listed")
	}
}

func TestForwardedEnvPassesOnlyNamedVariables(t *testing.T) {
	t.Setenv("SIRENS_ECHO_TEST_FORWARDED", "carried")
	t.Setenv("SIRENS_ECHO_TEST_WITHHELD", "secret")

	forwarded := forwardedEnv([]string{"SIRENS_ECHO_TEST_FORWARDED", "SIRENS_ECHO_TEST_ABSENT"})
	if len(forwarded) != 1 || forwarded[0] != "SIRENS_ECHO_TEST_FORWARDED=carried" {
		t.Fatalf("forwarded = %#v", forwarded)
	}
}

// liveMCPServer starts one streamable MCP server publishing a single tool.
func liveMCPServer(t *testing.T, name, tool string) string {
	t.Helper()
	server := mcp.NewServer(&mcp.Implementation{Name: name, Version: "1"}, nil)
	mcp.AddTool(
		server,
		&mcp.Tool{Name: tool, Description: "fixture tool"},
		func(context.Context, *mcp.CallToolRequest, struct{}) (
			*mcp.CallToolResult, any, error,
		) {
			return &mcp.CallToolResult{
				Content: []mcp.Content{&mcp.TextContent{Text: "ok"}},
			}, nil, nil
		},
	)
	handler := mcp.NewStreamableHTTPHandler(
		func(*http.Request) *mcp.Server { return server },
		&mcp.StreamableHTTPOptions{JSONResponse: true},
	)
	httpServer := httptest.NewServer(handler)
	t.Cleanup(httpServer.Close)
	return httpServer.URL
}

func TestMCPProviderServesReachableServersWhenOneIsDown(t *testing.T) {
	t.Parallel()
	reachable := liveMCPServer(t, "eco-test", "get_status")
	// Closed immediately, so the endpoint refuses connections.
	dead := httptest.NewServer(http.HandlerFunc(func(http.ResponseWriter, *http.Request) {}))
	dead.Close()

	session, err := (&MCPProvider{Servers: []MCPServerDefinition{
		{Name: "down", URL: dead.URL},
		{Name: "eco", URL: reachable},
	}}).Open(context.Background())
	if err != nil {
		t.Fatalf("Open with one server down: %v", err)
	}
	defer func() { _ = session.Close() }()

	if got := session.Unavailable(); len(got) != 1 || got[0] != "down" {
		t.Fatalf("unavailable = %#v, want [down]", got)
	}
	tools := session.Tools()
	if len(tools) != 1 || tools[0].Name != "eco__get_status" {
		t.Fatalf("tools = %#v, want the reachable server's tool", tools)
	}
}

func TestMCPProviderFailsWhenNoServerIsReachable(t *testing.T) {
	t.Parallel()
	dead := httptest.NewServer(http.HandlerFunc(func(http.ResponseWriter, *http.Request) {}))
	dead.Close()

	// A roster where nothing answered is a capability outage, not a partial one.
	if _, err := (&MCPProvider{Servers: []MCPServerDefinition{
		{Name: "down", URL: dead.URL},
	}}).Open(context.Background()); err == nil {
		t.Fatal("an entirely unreachable roster must fail the turn")
	}
}

func TestMCPProviderKeepsToolNameCollisionFatal(t *testing.T) {
	t.Parallel()
	first := liveMCPServer(t, "one", "status")
	second := liveMCPServer(t, "two", "status")

	// Same proxy name from two servers is a roster mistake, so it must not
	// degrade into silently dropping whichever lost.
	if _, err := (&MCPProvider{Servers: []MCPServerDefinition{
		{Name: "dup", URL: first},
		{Name: "dup", URL: second},
	}}).Open(context.Background()); err == nil {
		t.Fatal("a tool name collision must stay fatal")
	}
}

func TestMCPProviderClosesCleanupResponseSpan(t *testing.T) {
	mcpServer := mcp.NewServer(
		&mcp.Implementation{Name: "eco-test", Version: "1"},
		nil,
	)
	streamable := mcp.NewStreamableHTTPHandler(
		func(*http.Request) *mcp.Server { return mcpServer },
		&mcp.StreamableHTTPOptions{JSONResponse: true},
	)
	var deleteRequests atomic.Int32
	var headersMu sync.Mutex
	traceParents := make(map[string][]string)
	httpServer := httptest.NewServer(http.HandlerFunc(func(
		writer http.ResponseWriter,
		request *http.Request,
	) {
		headersMu.Lock()
		traceParents[request.Method] = append(
			traceParents[request.Method],
			request.Header.Get("traceparent"),
		)
		headersMu.Unlock()
		if request.Method == http.MethodDelete {
			deleteRequests.Add(1)
			http.Error(writer, "method not allowed", http.StatusMethodNotAllowed)
			return
		}
		streamable.ServeHTTP(writer, request)
	}))
	t.Cleanup(httpServer.Close)

	recorder := tracetest.NewSpanRecorder()
	traceProvider := sdktrace.NewTracerProvider(
		sdktrace.WithSpanProcessor(recorder),
	)
	t.Cleanup(func() {
		_ = traceProvider.Shutdown(context.Background())
	})
	httpClient := &http.Client{
		Timeout: time.Second,
		Transport: otelhttp.NewTransport(
			http.DefaultTransport,
			otelhttp.WithTracerProvider(traceProvider),
			otelhttp.WithPropagators(propagation.TraceContext{}),
		),
	}
	ctx, parent := traceProvider.Tracer("sirens-echo-test").Start(
		context.Background(),
		"community.turn",
	)
	parentContext := parent.SpanContext()

	provider := &MCPProvider{
		Servers:    []MCPServerDefinition{{Name: "eco", URL: httpServer.URL}},
		HTTPClient: httpClient,
	}
	session, err := provider.Open(ctx)
	if err != nil {
		parent.End()
		t.Fatalf("Open: %v", err)
	}
	// The view borrows the supervised session. Cleanup is the provider's, so
	// the DELETE lands on provider shutdown rather than at the end of a turn.
	if err := session.Close(); err != nil {
		parent.End()
		t.Fatalf("Close: %v", err)
	}
	if err := provider.Close(); err != nil {
		parent.End()
		t.Fatalf("provider Close: %v", err)
	}
	parent.End()

	if got := deleteRequests.Load(); got != 1 {
		t.Fatalf("DELETE requests = %d, want 1", got)
	}
	endedMethods := make(map[string]int)
	for _, span := range recorder.Ended() {
		if span.SpanKind() != trace.SpanKindClient {
			continue
		}
		method := spanStringAttribute(span.Attributes(), "http.request.method")
		if method != http.MethodPost && method != http.MethodDelete {
			continue
		}
		endedMethods[method]++
		if span.SpanContext().TraceID() != parentContext.TraceID() ||
			span.Parent().SpanID() != parentContext.SpanID() {
			t.Errorf("%s client span is outside the turn trace", method)
		}
	}
	if endedMethods[http.MethodPost] == 0 {
		t.Fatal("ordinary MCP POST client spans did not end")
	}
	if endedMethods[http.MethodDelete] != 1 {
		t.Fatalf(
			"ended MCP DELETE client spans = %d, want 1",
			endedMethods[http.MethodDelete],
		)
	}

	headersMu.Lock()
	defer headersMu.Unlock()
	for _, method := range []string{http.MethodPost, http.MethodDelete} {
		if len(traceParents[method]) == 0 {
			t.Fatalf("%s traceparent headers = %#v", method, traceParents[method])
		}
		for _, traceParent := range traceParents[method] {
			if !strings.Contains(traceParent, parentContext.TraceID().String()) {
				t.Errorf("%s traceparent = %q", method, traceParent)
			}
		}
	}
}

func TestToolResultTextRendersContentTheModelCanRead(t *testing.T) {
	t.Parallel()
	text, err := toolResultText(&mcp.CallToolResult{
		Content: []mcp.Content{
			&mcp.TextContent{Text: "first"},
			&mcp.TextContent{Text: "second"},
		},
	})
	if err != nil {
		t.Fatalf("toolResultText: %v", err)
	}
	if text != "first\nsecond" {
		t.Fatalf("text = %q, want the joined text parts", text)
	}
}

func TestToolResultTextFallsBackToStructuredContent(t *testing.T) {
	t.Parallel()
	text, err := toolResultText(&mcp.CallToolResult{
		StructuredContent: map[string]any{"status": "online"},
	})
	if err != nil {
		t.Fatalf("toolResultText: %v", err)
	}
	if !strings.Contains(text, "online") {
		t.Fatalf("text = %q, want the structured payload", text)
	}
}

func TestToolResultTextSurvivesBoundingAsPlainText(t *testing.T) {
	t.Parallel()
	// The regression: bounding cut a marshalled envelope, so an oversized
	// result reached the model as invalid JSON. Text has no structure to break.
	text, err := toolResultText(&mcp.CallToolResult{
		Content: []mcp.Content{
			&mcp.TextContent{Text: strings.Repeat("a", maxToolResultBytes*2)},
		},
	})
	if err != nil {
		t.Fatalf("toolResultText: %v", err)
	}
	if strings.HasPrefix(strings.TrimSpace(text), "{") {
		t.Fatal("a text result was rendered as a JSON envelope")
	}
	bounded, trimmed := boundToolResult(text)
	if !trimmed {
		t.Fatal("an oversized result was not bounded")
	}
	if !utf8.ValidString(bounded) {
		t.Fatal("bounding produced invalid UTF-8")
	}
}

func spanStringAttribute(attributes []attribute.KeyValue, key attribute.Key) string {
	for _, item := range attributes {
		if item.Key == key {
			return item.Value.AsString()
		}
	}
	return ""
}
