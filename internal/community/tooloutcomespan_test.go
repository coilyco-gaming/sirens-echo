package community

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"
	"time"

	"go.opentelemetry.io/otel/codes"
	metricnoop "go.opentelemetry.io/otel/metric/noop"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	"go.opentelemetry.io/otel/sdk/trace/tracetest"
)

// A member can see that a tool returned nothing, from the disclosure footer. An
// operator reading the trace could not. See sirens-echo#570.

// recordedAttribute reads one attribute off a recorded span.
func recordedAttribute(span sdktrace.ReadOnlySpan, key string) string {
	for _, attribute := range span.Attributes() {
		if string(attribute.Key) == key {
			return attribute.Value.Emit()
		}
	}
	return ""
}

// toolCallSpan drives one real tool round through Complete and returns the
// recorded mcp.tool.call span, so the assertion is on what a reader would see.
func toolCallSpan(t *testing.T, fixture FixtureTool) sdktrace.ReadOnlySpan {
	t.Helper()
	var round atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(
		writer http.ResponseWriter, request *http.Request,
	) {
		var body chatRequest
		_ = json.NewDecoder(request.Body).Decode(&body)
		writer.Header().Set("Content-Type", "application/json")
		if round.Add(1) == 1 {
			_, _ = writer.Write([]byte(`{"choices":[{"message":{"tool_calls":[{"id":"c1",` +
				`"type":"function","function":{"name":"find_trade","arguments":"{}"}}]}}]}`))
			return
		}
		_, _ = writer.Write([]byte(`{"choices":[{"message":{"content":"Answered."}}]}`))
	}))
	t.Cleanup(server.Close)

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

	client := ProxyClient{
		BaseURL:    server.URL,
		Model:      "selected-model",
		AuditRole:  "community",
		HTTPClient: &http.Client{Timeout: 2 * time.Second},
		Telemetry:  telemetry,
		Tools:      FixtureProvider{Pack: FixturePack{Tools: []FixtureTool{fixture}}},
	}
	if _, err := client.Complete(
		context.Background(), TurnPrompt{System: "system", Message: "user"}, "request",
	); err != nil {
		t.Fatalf("Complete: %v", err)
	}

	for _, ended := range recorder.Ended() {
		if ended.Name() == "mcp.tool.call" {
			return ended
		}
	}
	t.Fatal("no mcp.tool.call span was recorded, so this test asserts nothing")
	return nil
}

// The distinction sirens-echo#195 exists to protect: a call that returned rows
// and one that returned none must not read alike on the trace.
func TestATraceSaysWhetherAToolReturnedAnything(t *testing.T) {
	t.Parallel()
	for _, testCase := range []struct {
		name    string
		fixture FixtureTool
		outcome ToolOutcome
		bytes   string
	}{
		{
			"rows",
			FixtureTool{Name: "find_trade", Server: "eco", Result: "913 offers"},
			ToolOutcomeOK, "10",
		},
		{
			"nothing",
			FixtureTool{Name: "find_trade", Server: "eco", Result: ""},
			ToolOutcomeEmpty, "0",
		},
		{
			"a tool that reported failure",
			FixtureTool{Name: "find_trade", Server: "eco", Result: "boom", IsError: true},
			ToolOutcomeFailed, "4",
		},
	} {
		testCase := testCase
		t.Run(testCase.name, func(t *testing.T) {
			t.Parallel()
			span := toolCallSpan(t, testCase.fixture)
			if got := recordedAttribute(span, "mcp.tool.outcome"); got != string(testCase.outcome) {
				t.Errorf("mcp.tool.outcome = %q, want %q", got, testCase.outcome)
			}
			if got := recordedAttribute(span, "mcp.tool.result_bytes"); got != testCase.bytes {
				t.Errorf("mcp.tool.result_bytes = %q, want %q", got, testCase.bytes)
			}
		})
	}
}

// 51 tool calls reported failure over 12h and 1 span carried error status, so
// every generic error query sailed past them. See sirens-echo#873.
func TestAToolReportingFailureCarriesSpanErrorStatus(t *testing.T) {
	t.Parallel()
	failed := toolCallSpan(t, FixtureTool{
		Name: "find_trade", Server: "eco", Result: "boom", IsError: true,
	})
	if got := failed.Status().Code; got != codes.Error {
		t.Errorf("status code = %v, want %v", got, codes.Error)
	}
	spec := exceptionFor(exceptionMCPToolReportedError)
	if got := failed.Status().Description; got != spec.message {
		t.Errorf("status description = %q, want %q", got, spec.message)
	}
	// The reason was null on all 51, which is the second half of the issue.
	if got := recordedAttribute(failed, "error.type"); got != spec.typeName {
		t.Errorf("error.type = %q, want %q", got, spec.typeName)
	}
	if got := recordedAttribute(failed, "error.outcome"); got != spec.outcome {
		t.Errorf("error.outcome = %q, want %q", got, spec.outcome)
	}
}

// A working call must stay unset, or the status stops meaning anything.
func TestASucceedingToolCallCarriesNoErrorStatus(t *testing.T) {
	t.Parallel()
	ok := toolCallSpan(t, FixtureTool{Name: "find_trade", Server: "eco", Result: "913 offers"})
	if got := ok.Status().Code; got == codes.Error {
		t.Errorf("a successful tool call carried error status")
	}
	if got := recordedAttribute(ok, "error.type"); got != "" {
		t.Errorf("error.type = %q on a successful call", got)
	}
}

// A deadline and any other transport failure read alike until now, so a trace
// could not confirm the 30s MCP timeout the issue suspected.
func TestATimedOutToolCallIsNamedApartFromOtherFailures(t *testing.T) {
	t.Parallel()
	timedOut := mcpCallFailure(fmt.Errorf("call eco: %w", context.DeadlineExceeded))
	if timedOut != exceptionMCPToolCallTimedOut {
		t.Errorf("a deadline classified as %v", exceptionFor(timedOut).typeName)
	}
	other := mcpCallFailure(errors.New("connection reset"))
	if other != exceptionMCPToolCallFailed {
		t.Errorf("a transport failure classified as %v", exceptionFor(other).typeName)
	}
}
