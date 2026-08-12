package community

import (
	"context"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net/url"
	"os"
	"path"
	"time"

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/exporters/otlp/otlpmetric/otlpmetrichttp"
	"go.opentelemetry.io/otel/exporters/otlp/otlptrace/otlptracehttp"
	"go.opentelemetry.io/otel/metric"
	metricnoop "go.opentelemetry.io/otel/metric/noop"
	"go.opentelemetry.io/otel/propagation"
	sdkmetric "go.opentelemetry.io/otel/sdk/metric"
	"go.opentelemetry.io/otel/sdk/resource"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	"go.opentelemetry.io/otel/trace"
	tracenoop "go.opentelemetry.io/otel/trace/noop"
)

const telemetryScope = "coilyco-gaming/sirens-echo/internal/community"

// Telemetry joins JSON logs, traces, and metrics for one Echo process.
// Kubernetes stdout and OTLP export carry them to the same SigNoZ stack.
type Telemetry struct {
	logger               *slog.Logger
	tracer               trace.Tracer
	turns                metric.Int64Counter
	turnDuration         metric.Float64Histogram
	modelCalls           metric.Int64Counter
	toolCalls            metric.Int64Counter
	admissions           metric.Int64Counter
	accessChecks         metric.Int64Counter
	failures             metric.Int64Counter
	jobs                 metric.Int64Counter
	healthRequests       metric.Int64Counter
	readinessDuration    metric.Float64Histogram
	readinessState       metric.Int64Gauge
	readinessLastSuccess metric.Int64Gauge
	traceSDK             *sdktrace.TracerProvider
	metricSDK            *sdkmetric.MeterProvider
	traceProvider        trace.TracerProvider
	propagator           propagation.TextMapPropagator
}

// NewTelemetry initializes OTLP/HTTP export and a JSON stdout logger.
func NewTelemetry(ctx context.Context, cfg Config) (*Telemetry, error) {
	traceEndpoint, err := otlpSignalURL(cfg.OTLPEndpoint, "traces")
	if err != nil {
		return nil, err
	}
	metricEndpoint, err := otlpSignalURL(cfg.OTLPEndpoint, "metrics")
	if err != nil {
		return nil, err
	}
	traceExporter, err := otlptracehttp.New(
		ctx,
		otlptracehttp.WithEndpointURL(traceEndpoint),
	)
	if err != nil {
		return nil, err
	}
	metricExporter, err := newMetricExporter(ctx, metricEndpoint)
	if err != nil {
		_ = traceExporter.Shutdown(ctx)
		return nil, err
	}
	res, err := resource.New(
		ctx,
		resource.WithAttributes(
			attribute.String("service.name", valueOrDefault(cfg.InstanceName, defaultInstanceName)),
			attribute.String("service.namespace", "sirens"),
			attribute.String("agent.role", cfg.Definition.AuditRole),
			attribute.String("agent.attribution", cfg.Definition.Identity),
		),
	)
	if err != nil {
		_ = traceExporter.Shutdown(ctx)
		_ = metricExporter.Shutdown(ctx)
		return nil, err
	}
	traceSDK := sdktrace.NewTracerProvider(
		sdktrace.WithBatcher(traceExporter),
		sdktrace.WithResource(res),
	)
	metricSDK := sdkmetric.NewMeterProvider(
		sdkmetric.WithReader(sdkmetric.NewPeriodicReader(
			metricExporter,
			sdkmetric.WithInterval(15*time.Second),
		)),
		sdkmetric.WithResource(res),
	)
	otel.SetTracerProvider(traceSDK)
	otel.SetMeterProvider(metricSDK)
	otel.SetTextMapPropagator(propagation.NewCompositeTextMapPropagator(
		propagation.TraceContext{},
		propagation.Baggage{},
	))
	logger := slog.New(slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{
		Level: slog.LevelInfo,
	}))
	telemetry, err := newTelemetry(logger, traceSDK, metricSDK)
	if err != nil {
		_ = metricSDK.Shutdown(ctx)
		_ = traceSDK.Shutdown(ctx)
		return nil, err
	}
	telemetry.traceSDK = traceSDK
	telemetry.metricSDK = metricSDK
	return telemetry, nil
}

func newMetricExporter(
	ctx context.Context,
	endpoint string,
) (*otlpmetrichttp.Exporter, error) {
	return otlpmetrichttp.New(
		ctx,
		otlpmetrichttp.WithEndpointURL(endpoint),
		otlpmetrichttp.WithTemporalitySelector(sdkmetric.DeltaTemporalitySelector),
	)
}

func otlpSignalURL(base, signal string) (string, error) {
	parsed, err := url.Parse(base)
	if err != nil || (parsed.Scheme != "http" && parsed.Scheme != "https") ||
		parsed.Host == "" {
		return "", fmt.Errorf("invalid OTLP endpoint %q", base)
	}
	parsed.Path = path.Join(parsed.Path, "v1", signal)
	return parsed.String(), nil
}

func newTelemetry(
	logger *slog.Logger,
	tracerProvider trace.TracerProvider,
	meterProvider metric.MeterProvider,
) (*Telemetry, error) {
	meter := meterProvider.Meter(telemetryScope)
	turns, err := meter.Int64Counter("sirens_echo.turns")
	if err != nil {
		return nil, err
	}
	turnDuration, err := meter.Float64Histogram(
		"sirens_echo.turn.duration",
		metric.WithUnit("ms"),
	)
	if err != nil {
		return nil, err
	}
	modelCalls, err := meter.Int64Counter("sirens_echo.model.calls")
	if err != nil {
		return nil, err
	}
	toolCalls, err := meter.Int64Counter("sirens_echo.tool.calls")
	if err != nil {
		return nil, err
	}
	admissions, err := meter.Int64Counter("sirens_echo.admissions")
	if err != nil {
		return nil, err
	}
	accessChecks, err := meter.Int64Counter("sirens_echo.access.checks")
	if err != nil {
		return nil, err
	}
	failures, err := meter.Int64Counter("sirens_echo.failures")
	if err != nil {
		return nil, err
	}
	jobs, err := meter.Int64Counter("sirens_echo.jobs")
	if err != nil {
		return nil, err
	}
	healthRequests, err := meter.Int64Counter("sirens_echo.health.requests")
	if err != nil {
		return nil, err
	}
	readinessDuration, err := meter.Float64Histogram(
		"sirens_echo.readiness.duration",
		metric.WithUnit("ms"),
	)
	if err != nil {
		return nil, err
	}
	readinessState, err := meter.Int64Gauge(
		"sirens_echo.readiness.state",
		metric.WithUnit("1"),
	)
	if err != nil {
		return nil, err
	}
	readinessLastSuccess, err := meter.Int64Gauge(
		"sirens_echo.readiness.last_success",
		metric.WithUnit("s"),
	)
	if err != nil {
		return nil, err
	}
	return &Telemetry{
		logger:               logger,
		tracer:               tracerProvider.Tracer(telemetryScope),
		turns:                turns,
		turnDuration:         turnDuration,
		modelCalls:           modelCalls,
		toolCalls:            toolCalls,
		admissions:           admissions,
		accessChecks:         accessChecks,
		failures:             failures,
		jobs:                 jobs,
		healthRequests:       healthRequests,
		readinessDuration:    readinessDuration,
		readinessState:       readinessState,
		readinessLastSuccess: readinessLastSuccess,
		traceProvider:        tracerProvider,
		propagator: propagation.NewCompositeTextMapPropagator(
			propagation.TraceContext{},
			propagation.Baggage{},
		),
	}, nil
}

func telemetryOrNoop(telemetry *Telemetry) *Telemetry {
	if telemetry != nil {
		return telemetry
	}
	logger := slog.New(slog.NewJSONHandler(io.Discard, nil))
	noop, _ := newTelemetry(
		logger,
		tracenoop.NewTracerProvider(),
		metricnoop.NewMeterProvider(),
	)
	return noop
}

// StartSpan starts a child span using the service instrumentation scope.
func (t *Telemetry) StartSpan(
	ctx context.Context,
	name string,
	attributes ...attribute.KeyValue,
) (context.Context, trace.Span) {
	return t.tracer.Start(ctx, name, trace.WithAttributes(attributes...))
}

// Info emits one trace-correlated metadata log.
func (t *Telemetry) Info(ctx context.Context, message string, attrs ...slog.Attr) {
	t.log(ctx, slog.LevelInfo, message, attrs...)
}

// Error emits one trace-correlated JSON error log.
func (t *Telemetry) Error(ctx context.Context, message string, attrs ...slog.Attr) {
	t.log(ctx, slog.LevelError, message, attrs...)
}

func (t *Telemetry) log(
	ctx context.Context,
	level slog.Level,
	message string,
	attrs ...slog.Attr,
) {
	spanContext := trace.SpanContextFromContext(ctx)
	if spanContext.IsValid() {
		attrs = append(
			attrs,
			slog.String("trace_id", spanContext.TraceID().String()),
			slog.String("span_id", spanContext.SpanID().String()),
		)
	}
	// A job id joins trace_id and span_id as a row field, so a job's whole log
	// history is retrievable by id. See docs/sirens-echo-jobs-lifecycle.md.
	if id := JobIDFromContext(ctx); id != "" {
		attrs = append(attrs, slog.String("job_id", id))
	}
	t.logger.LogAttrs(ctx, level, message, attrs...)
}

// RecordJob counts a job reaching a state. Kind and state are closed sets, so
// neither is an unbounded label.
func (t *Telemetry) RecordJob(ctx context.Context, kind, state string) {
	t.jobs.Add(ctx, 1, metric.WithAttributes(
		attribute.String("kind", kind),
		attribute.String("state", state),
	))
}

// StartJobSpan opens the span every one of a job's spans descends from, and
// puts the job id in the context so logs carry it too.
func (t *Telemetry) StartJobSpan(
	ctx context.Context,
	job Job,
) (context.Context, trace.Span) {
	ctx = ContextWithJobID(ctx, job.ID)
	return t.StartSpan(
		ctx,
		"job.execute",
		attribute.String("sirens_echo.job.id", job.ID),
		attribute.String("sirens_echo.job.kind", job.Kind),
		attribute.String("sirens_echo.job.transport", job.Origin.Transport),
		attribute.Int("sirens_echo.job.attempt", job.Attempts),
	)
}

// RecordTurn records the terminal state and latency of one accepted summon.
func (t *Telemetry) RecordTurn(
	ctx context.Context,
	outcome string,
	duration time.Duration,
) {
	options := metric.WithAttributes(attribute.String("outcome", outcome))
	t.turns.Add(ctx, 1, options)
	t.turnDuration.Record(ctx, float64(duration.Microseconds())/1000, options)
}

// RecordModelCall records one Agent Proxy round.
func (t *Telemetry) RecordModelCall(ctx context.Context, outcome string) {
	t.modelCalls.Add(
		ctx,
		1,
		metric.WithAttributes(attribute.String("outcome", outcome)),
	)
}

// RecordToolCall records one completed or failed MCP invocation.
func (t *Telemetry) RecordToolCall(
	ctx context.Context,
	server, tool, outcome string,
) {
	t.toolCalls.Add(
		ctx,
		1,
		metric.WithAttributes(
			attribute.String("mcp.server.name", server),
			attribute.String("mcp.tool.name", tool),
			attribute.String("outcome", outcome),
		),
	)
}

// RecordAdmission records one admission decision. Both labels are closed sets,
// so a flood cannot expand metric cardinality through member-supplied values.
func (t *Telemetry) RecordAdmission(ctx context.Context, outcome, transport string) {
	t.admissions.Add(
		ctx,
		1,
		metric.WithAttributes(
			attribute.String("outcome", outcome),
			attribute.String("transport", transport),
		),
	)
}

// RecordAccess records one allowlist verdict. The reason is a closed set, so
// no guild, channel, or member identifier reaches a metric label.
func (t *Telemetry) RecordAccess(ctx context.Context, reason string) {
	t.accessChecks.Add(ctx, 1, metric.WithAttributes(attribute.String("reason", reason)))
}

// RecordFailure records the stage where an accepted turn failed.
func (t *Telemetry) RecordFailure(ctx context.Context, stage string) {
	t.failures.Add(
		ctx,
		1,
		metric.WithAttributes(attribute.String("stage", stage)),
	)
}

// RecordHealth counts one metrics-only health request with closed-set labels.
func (t *Telemetry) RecordHealth(ctx context.Context, endpoint, outcome string) {
	t.healthRequests.Add(
		ctx,
		1,
		metric.WithAttributes(
			attribute.String("endpoint", endpoint),
			attribute.String("outcome", outcome),
		),
	)
}

// RecordReadiness updates structural route readiness without creating an event.
func (t *Telemetry) RecordReadiness(
	ctx context.Context,
	outcome readinessOutcome,
	duration time.Duration,
) {
	options := metric.WithAttributes(attribute.String("outcome", string(outcome)))
	t.readinessDuration.Record(ctx, float64(duration.Microseconds())/1000, options)
	state := int64(0)
	if outcome == readinessReady {
		state = 1
		t.readinessLastSuccess.Record(ctx, time.Now().Unix())
	}
	t.readinessState.Record(ctx, state)
}

// Close flushes metrics and traces before process exit.
func (t *Telemetry) Close(ctx context.Context) error {
	var errs []error
	if t.metricSDK != nil {
		errs = append(errs, t.metricSDK.Shutdown(ctx))
	}
	if t.traceSDK != nil {
		errs = append(errs, t.traceSDK.Shutdown(ctx))
	}
	return errors.Join(errs...)
}
