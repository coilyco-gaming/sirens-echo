package ingest

import (
	"context"
	"log/slog"
)

// Logger is the subset of the runtime's telemetry this package needs, declared
// here so ingress depends on a shape rather than on the community package.
type Logger interface {
	Info(ctx context.Context, message string, attrs ...slog.Attr)
	Error(ctx context.Context, message string, attrs ...slog.Attr)
}

// Observer records what ingress did. Every label it takes is a closed set, so
// a flood cannot expand metric cardinality. See docs/sirens-echo-admission.md.
type Observer interface {
	// Accepted counts an ask that entered the queue.
	Accepted(ctx context.Context, surface string)
	// Shed counts an ask the bound dropped to make room for a newer one.
	Shed(ctx context.Context, surface string)
	// Depth samples the backlog, which is also what widening reads.
	Depth(ctx context.Context, depth int)
}

// discard satisfies Observer for a deployment that exports no metrics, so no
// call site needs a nil check.
type discard struct{}

func (discard) Accepted(context.Context, string) {}
func (discard) Shed(context.Context, string)     {}
func (discard) Depth(context.Context, int)       {}

// quiet satisfies Logger the same way.
type quiet struct{}

func (quiet) Info(context.Context, string, ...slog.Attr)  {}
func (quiet) Error(context.Context, string, ...slog.Attr) {}
