package community

import (
	"context"
	"log/slog"
	"testing"
	"time"

	sdkmetric "go.opentelemetry.io/otel/sdk/metric"
	"go.opentelemetry.io/otel/sdk/metric/metricdata"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
)

// admissionOutcomes reads what sirens_echo.admissions actually recorded, so
// the assertion is on what an operator would query.
func admissionOutcomes(t *testing.T, reader *sdkmetric.ManualReader) map[string]int64 {
	t.Helper()
	counted := make(map[string]int64)
	metric := metricByName(t, collectMetrics(t, reader), "sirens_echo.admissions")
	sums, ok := metric.Data.(metricdata.Sum[int64])
	if !ok {
		t.Fatalf("sirens_echo.admissions is %T, want an int64 sum", metric.Data)
	}
	for _, point := range sums.DataPoints {
		outcome, found := point.Attributes.Value("outcome")
		if !found {
			t.Fatal("an admission point carries no outcome")
		}
		counted[outcome.AsString()] += point.Value
	}
	return counted
}

func meteredAgent(t *testing.T) (*Agent, *sdkmetric.ManualReader) {
	t.Helper()
	reader := sdkmetric.NewManualReader()
	provider := sdkmetric.NewMeterProvider(sdkmetric.WithReader(reader))
	traces := sdktrace.NewTracerProvider()
	t.Cleanup(func() {
		_ = provider.Shutdown(context.Background())
		_ = traces.Shutdown(context.Background())
	})
	telemetry, err := newTelemetry(slog.New(slog.DiscardHandler), traces, provider)
	if err != nil {
		t.Fatalf("newTelemetry: %v", err)
	}
	agent := &Agent{
		cfg: Config{
			Definition:     Definition{MaxContextMessages: 12},
			ExecutionSlots: 1,
			QueueTimeout:   20 * time.Millisecond,
			RequestTimeout: 5 * time.Second,
		},
		systemPrompt: "neutral model policy and local knowledge",
		telemetry:    telemetry,
	}
	agent.ensureRuntimeDefaults()
	return agent, reader
}

// The finding behind sirens-echo#1083. Two mechanisms shared one label, on the
// same transport, so the measurement could not answer its own first question.
func TestABacklogRefusalAndASlotWaitAreSeparateOutcomes(t *testing.T) {
	t.Parallel()

	limiter := newRateLimiter(RateLimitPolicy{MaxPending: 1}, 16)
	if got := limiter.Admit(admissionRequest{Queued: true}).Outcome; got != admissionAccepted {
		t.Fatalf("first admission = %q", got)
	}
	backlog := limiter.Admit(admissionRequest{Queued: true}).Outcome
	if backlog != admissionBacklog {
		t.Fatalf("a full backlog reported %q, want %q", backlog, admissionBacklog)
	}

	agent, reader := meteredAgent(t)
	client := blockingClient{arrived: make(chan struct{}, 4), release: make(chan struct{})}
	agent.completions = client
	go func() {
		_ = agent.runSerialized(context.Background(), &httpTurn{
			requestID: "holder",
			current:   TranscriptEntry{Author: "member", Content: "are you ready?"},
		})
	}()
	awaitArrivals(t, client, 1)
	if err := agent.runSerialized(context.Background(), &httpTurn{
		requestID: "waited",
		current:   TranscriptEntry{Author: "member", Content: "are you ready?"},
	}); err == nil {
		t.Fatal("the second turn was admitted while the only slot was held")
	}
	close(client.release)

	counted := admissionOutcomes(t, reader)
	if counted[string(admissionSlotWait)] != 1 {
		t.Errorf(
			"a turn that gave up waiting recorded %v, want one %q",
			counted, admissionSlotWait,
		)
	}
	if counted[string(admissionBacklog)] != 0 {
		t.Errorf("a slot wait was recorded as a backlog refusal: %v", counted)
	}
	// The label the two used to share. Either one still emitting it means an
	// operator cannot tell a full backlog from a slow turn.
	if counted["denied_queue"] != 0 {
		t.Errorf("denied_queue is still emitted, so the two are still one label: %v", counted)
	}
}
