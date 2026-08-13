package community

import (
	"context"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"testing"
	"time"

	metricnoop "go.opentelemetry.io/otel/metric/noop"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	"go.opentelemetry.io/otel/sdk/trace/tracetest"
)

// A rejection was attributable to the model and not to the check that made it,
// and seconds of every turn had no span at all. See sirens-echo#652.

// answeringClient returns one fixed reply, so a test picks which check refuses.
type answeringClient struct{ reply string }

func (c answeringClient) Complete(
	context.Context, TurnPrompt, string,
) (CompletionResult, error) {
	return CompletionResult{Content: c.reply}, nil
}

// recordedTurn runs one whole turn against a recording tracer and returns every
// span it ended, so an assertion is on what a reader would see.
func recordedTurn(t *testing.T, reply string) ([]sdktrace.ReadOnlySpan, *httpTurn) {
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

	agent := &Agent{
		cfg:          Config{Definition: Definition{MaxContextMessages: 12}},
		completions:  answeringClient{reply: reply},
		systemPrompt: "neutral model policy and local knowledge",
		telemetry:    telemetry,
		slots:        make(chan struct{}, 1),
	}
	agent.ensureRuntimeDefaults()
	turn := &httpTurn{requestID: "staged-turn", current: TranscriptEntry{
		Author: "member", Content: "what is happening?",
	}}
	_ = agent.runTurn(context.Background(), turn, nil)
	return recorder.Ended(), turn
}

func endedSpan(spans []sdktrace.ReadOnlySpan, name string) sdktrace.ReadOnlySpan {
	for _, span := range spans {
		if span.Name() == name {
			return span
		}
	}
	return nil
}

// The headline for sirens-echo#651. A refused reply names the check that
// refused, so the rejection stops being attributable to the model.
func TestARefusedReplyNamesTheCheckThatRefusedIt(t *testing.T) {
	t.Parallel()
	spans, _ := recordedTurn(t, observedMarkupReply)

	validate := endedSpan(spans, "response.validate")
	if validate == nil {
		t.Fatal("no response.validate span, so this test asserts nothing")
	}
	if got := recordedAttribute(validate, "response.check"); got != replyCheckToolCallMarkup {
		t.Errorf("response.check = %q, want %q", got, replyCheckToolCallMarkup)
	}
}

// A reply that passes says so, so a reader never has to read absence as
// success. Every turn carries the attribute.
func TestAnAcceptedReplyNamesNoCheck(t *testing.T) {
	t.Parallel()
	spans, _ := recordedTurn(t, "Twelve rows are listed.")

	validate := endedSpan(spans, "response.validate")
	if validate == nil {
		t.Fatal("no response.validate span, so this test asserts nothing")
	}
	if got := recordedAttribute(validate, "response.check"); got != replyCheckNone {
		t.Errorf("response.check = %q, want %q", got, replyCheckNone)
	}
}

// The order of the checks is the contract. Restructuring them into a slice must
// not have reordered the one that runs first.
func TestTheChecksKeepTheirOrder(t *testing.T) {
	t.Parallel()
	agent := &Agent{
		cfg:         Config{Definition: Definition{Identity: "Sirens Echo of Coilyco"}},
		telemetry:   telemetryOrNoop(nil),
		identifiers: NewIdentifierGuard(Config{}, nil),
	}
	// Carries tool-call markup and an ungrounded claim. Markup is checked
	// first, so that is the name a reader must get.
	_, refused, err := agent.runReplyChecks(
		observedMarkupReply, TurnPrompt{}, CompletionResult{},
	)
	if err == nil {
		t.Fatal("a reply carrying tool-call markup was accepted")
	}
	if refused != replyCheckToolCallMarkup {
		t.Errorf("refused = %q, want the first check %q", refused, replyCheckToolCallMarkup)
	}
}

// Delivery is recorded rather than inferred from the absence of an error, which
// is what made an undelivered reply impossible to count.
func TestADeliveredReplyIsRecordedAsDelivered(t *testing.T) {
	t.Parallel()
	var recorded strings.Builder
	telemetry, err := newTelemetry(
		slog.New(slog.NewJSONHandler(&recorded, nil)),
		sdktrace.NewTracerProvider(),
		metricnoop.NewMeterProvider(),
	)
	if err != nil {
		t.Fatalf("telemetry: %v", err)
	}
	agent := &Agent{telemetry: telemetry}
	agent.ensureRuntimeDefaults()
	turn := &httpTurn{requestID: "delivered-turn"}

	if err := agent.sendReply(context.Background(), turn, "Twelve rows."); err != nil {
		t.Fatalf("sendReply: %v", err)
	}

	if !strings.Contains(recorded.String(), "turn.reply.delivered") {
		t.Errorf("no delivery was recorded:\n%s", recorded.String())
	}
}

// A send that failed records no delivery, or the count means nothing.
func TestAnUndeliveredReplyIsNotRecordedAsDelivered(t *testing.T) {
	t.Parallel()
	var recorded strings.Builder
	telemetry, err := newTelemetry(
		slog.New(slog.NewJSONHandler(&recorded, nil)),
		sdktrace.NewTracerProvider(),
		metricnoop.NewMeterProvider(),
	)
	if err != nil {
		t.Fatalf("telemetry: %v", err)
	}
	agent := &Agent{telemetry: telemetry}
	agent.ensureRuntimeDefaults()

	err = agent.sendReply(context.Background(), &undeliverableTurn{}, "Twelve rows.")

	if err == nil {
		t.Fatal("a refusing transport reported success")
	}
	if strings.Contains(recorded.String(), "turn.reply.delivered") {
		t.Errorf("a failed send was recorded as delivered:\n%s", recorded.String())
	}
}

// The rejection that ends a turn as model_failed is the completion layer's own
// contract check, and it used to record only an attempt count. See #651.
func TestARepairRecordsWhatItRefused(t *testing.T) {
	t.Parallel()
	// The exact answer sirens-echo#651 reports, which is first person and so
	// refused under the neutral profile.
	const capabilityAnswer = "No, I do not have access to an AOS general " +
		"knowledge base. My available tools are limited to Eco server data."
	var recorded strings.Builder
	server := httptest.NewServer(http.HandlerFunc(func(
		writer http.ResponseWriter, _ *http.Request,
	) {
		writer.Header().Set("Content-Type", "application/json")
		_, _ = writer.Write([]byte(`{"choices":[{"message":{"content":` +
			strconv.Quote(capabilityAnswer) + `}}]}`))
	}))
	t.Cleanup(server.Close)
	telemetry, err := newTelemetry(
		slog.New(slog.NewJSONHandler(&recorded, nil)),
		sdktrace.NewTracerProvider(),
		metricnoop.NewMeterProvider(),
	)
	if err != nil {
		t.Fatalf("telemetry: %v", err)
	}

	_, err = ProxyClient{
		BaseURL:       server.URL,
		Model:         "selected-model",
		ResponseStyle: ResponseStyleNeutral,
		HTTPClient:    &http.Client{Timeout: 2 * time.Second},
		Telemetry:     telemetry,
	}.Complete(context.Background(), TurnPrompt{System: "system", Message: "can you read the kb"}, "request")

	if err == nil {
		t.Fatal("a first-person reply was accepted under the neutral profile")
	}
	logged := recorded.String()
	repair := loggedLine(logged, "model.response.repair")
	if repair == "" {
		t.Fatalf("no repair was recorded:\n%s", logged)
	}
	// The whole point: the reason, on the repair line itself rather than only
	// on the line recording that the turn gave up.
	if !strings.Contains(repair, "first-person or collective voice") {
		t.Errorf("the repair does not say what it refused:\n%s", repair)
	}
	gaveUp := loggedLine(logged, "model.response.refused")
	if !strings.Contains(gaveUp, "first-person or collective voice") {
		t.Errorf("giving up recorded no reason:\n%s", logged)
	}
}

// loggedLine is the one JSON record carrying a message, so an assertion names
// the record it is about rather than the whole stream.
func loggedLine(logged, message string) string {
	for _, line := range strings.Split(logged, "\n") {
		if strings.Contains(line, `"`+message+`"`) {
			return line
		}
	}
	return ""
}
