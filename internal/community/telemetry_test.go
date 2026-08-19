package community

import (
	"context"
	"testing"

	sdkmetric "go.opentelemetry.io/otel/sdk/metric"
	"go.opentelemetry.io/otel/sdk/metric/metricdata"
)

func TestMetricExporterUsesDeltaTemporality(t *testing.T) {
	t.Parallel()
	exporter, err := newMetricExporter(
		context.Background(),
		"http://collector:4318/v1/metrics",
	)
	if err != nil {
		t.Fatalf("newMetricExporter: %v", err)
	}
	t.Cleanup(func() {
		if err := exporter.Shutdown(context.Background()); err != nil {
			t.Errorf("shutdown metric exporter: %v", err)
		}
	})

	for _, test := range []struct {
		name string
		kind sdkmetric.InstrumentKind
		want metricdata.Temporality
	}{
		{
			name: "counter",
			kind: sdkmetric.InstrumentKindCounter,
			want: metricdata.DeltaTemporality,
		},
		{
			name: "histogram",
			kind: sdkmetric.InstrumentKindHistogram,
			want: metricdata.DeltaTemporality,
		},
		{
			name: "observable counter",
			kind: sdkmetric.InstrumentKindObservableCounter,
			want: metricdata.DeltaTemporality,
		},
		{
			name: "gauge",
			kind: sdkmetric.InstrumentKindGauge,
			want: metricdata.CumulativeTemporality,
		},
		{
			name: "up-down counter",
			kind: sdkmetric.InstrumentKindUpDownCounter,
			want: metricdata.CumulativeTemporality,
		},
	} {
		test := test
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			if got := exporter.Temporality(test.kind); got != test.want {
				t.Fatalf("temporality = %s, want %s", got, test.want)
			}
		})
	}
}

func TestOTLPSignalURLAppendsProtocolPath(t *testing.T) {
	t.Parallel()
	for _, test := range []struct {
		name   string
		base   string
		signal string
		want   string
	}{
		{
			name:   "collector root",
			base:   "http://collector:4318",
			signal: "traces",
			want:   "http://collector:4318/v1/traces",
		},
		{
			name:   "collector base path",
			base:   "https://collector.example/otlp/",
			signal: "metrics",
			want:   "https://collector.example/otlp/v1/metrics",
		},
	} {
		test := test
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			got, err := otlpSignalURL(test.base, test.signal)
			if err != nil {
				t.Fatalf("otlpSignalURL: %v", err)
			}
			if got != test.want {
				t.Fatalf("URL = %q, want %q", got, test.want)
			}
		})
	}
}

func TestOTLPSignalURLRejectsInvalidBase(t *testing.T) {
	t.Parallel()
	if _, err := otlpSignalURL("collector:4318", "traces"); err == nil {
		t.Fatal("expected invalid endpoint error")
	}
}

// collectHistogram records one value through the configured views and returns
// the bucket boundaries and counts the reader saw for that instrument.
func collectHistogram(t *testing.T, name string, value float64) metricdata.HistogramDataPoint[float64] {
	t.Helper()
	reader := sdkmetric.NewManualReader()
	provider := sdkmetric.NewMeterProvider(
		sdkmetric.WithReader(reader),
		sdkmetric.WithView(histogramViews()...),
	)
	instrument, err := provider.Meter(telemetryScope).Float64Histogram(name)
	if err != nil {
		t.Fatalf("create %s: %v", name, err)
	}
	instrument.Record(context.Background(), value)
	var collected metricdata.ResourceMetrics
	if err := reader.Collect(context.Background(), &collected); err != nil {
		t.Fatalf("collect: %v", err)
	}
	for _, scope := range collected.ScopeMetrics {
		for _, recorded := range scope.Metrics {
			if recorded.Name != name {
				continue
			}
			histogram, ok := recorded.Data.(metricdata.Histogram[float64])
			if !ok {
				t.Fatalf("%s is %T, want a float histogram", name, recorded.Data)
			}
			return histogram.DataPoints[0]
		}
	}
	t.Fatalf("%s was never collected", name)
	return metricdata.HistogramDataPoint[float64]{}
}

// The bug: the SDK defaults top out at 10s, so every turn landed in the
// overflow bucket and p50 and p99 both reported the 10000 boundary. See #976.
func TestTurnDurationHistogramMeasuresAboveTenSeconds(t *testing.T) {
	t.Parallel()
	for _, name := range []string{
		"sirens_echo.turn.duration",
		"sirens_echo.coalesce.turn.duration",
	} {
		name := name
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			// Between the measured p50 of 17.2s and p95 of 181s, so a default
			// boundary set puts it in the overflow bucket and this one does not.
			point := collectHistogram(t, name, 60_000)
			top := point.Bounds[len(point.Bounds)-1]
			if top < 300_000 {
				t.Errorf("top boundary is %v ms, below the 5m request ceiling, so a "+
					"turn that runs the clock out is indistinguishable from one that "+
					"nearly did", top)
			}
			if overflow := point.BucketCounts[len(point.BucketCounts)-1]; overflow != 0 {
				t.Errorf("a 60s turn landed in the overflow bucket, so a percentile " +
					"over it reports a boundary rather than a duration")
			}
		})
	}
}

// A batch is bounded by the wide batch size, so the default first bucket held
// every batch and 1 read the same as 4. See #976.
func TestBatchSizeHistogramSeparatesSmallBatches(t *testing.T) {
	t.Parallel()
	point := collectHistogram(t, "sirens_echo.coalesce.batch.size", 1)
	if len(point.Bounds) == 0 || point.Bounds[0] != 1 {
		t.Fatalf("first boundary is %v, want 1 so a batch of one is its own bucket", point.Bounds)
	}
	// Bounds are inclusive upper edges, so a batch of 1 belongs to the first
	// bucket and nothing else may have been counted.
	if point.BucketCounts[0] != 1 {
		t.Errorf("a batch of 1 landed outside the first bucket: counts %v", point.BucketCounts)
	}
}
