package coalesce

import (
	"context"
	"log/slog"
	"time"
)

// Turn outcomes. A closed set, because it reaches a metric label and an open
// one would let a failure mode expand cardinality.
const (
	OutcomeServed    = "served"
	OutcomeFailed    = "failed"
	OutcomeAbandoned = "abandoned"
)

// Logger is the subset of the runtime's telemetry this package needs.
type Logger interface {
	Info(ctx context.Context, message string, attrs ...slog.Attr)
	Error(ctx context.Context, message string, attrs ...slog.Attr)
}

// Observer records what the coalescer did. These four series are what a
// dashboard needs, emitted now so nothing has to be instrumented later.
type Observer interface {
	// Depth samples the backlog, which is also what widening reads.
	Depth(ctx context.Context, depth int)
	// Batch records how many asks one turn absorbed, so coalescing is visible
	// as a distribution rather than as an average.
	Batch(ctx context.Context, size int)
	// Turn records one attempt's latency against the deadline.
	Turn(ctx context.Context, outcome string, tier Tier, took time.Duration)
	// Escalated counts batches that reached the wider model, which is the
	// series that says whether escalation is rare or load-bearing.
	Escalated(ctx context.Context)
	// DeadLettered counts batches the ladder gave up on.
	DeadLettered(ctx context.Context)
}

type discard struct{}

func (discard) Depth(context.Context, int)                        {}
func (discard) Batch(context.Context, int)                        {}
func (discard) Turn(context.Context, string, Tier, time.Duration) {}
func (discard) Escalated(context.Context)                         {}
func (discard) DeadLettered(context.Context)                      {}

type quiet struct{}

func (quiet) Info(context.Context, string, ...slog.Attr)  {}
func (quiet) Error(context.Context, string, ...slog.Attr) {}
