package community

import (
	"context"
	"fmt"
	"log/slog"
	"strings"

	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/trace"
)

// The gate that makes deny mean something. See docs/sirens-echo-content-gate.md.

// contentGateFailure names how the gate broke. It reaches a span name, so it is
// a closed vocabulary and never carries model output.
type contentGateFailure string

const (
	contentGateHealthy            contentGateFailure = ""
	contentGateFailedModel        contentGateFailure = "model"
	contentGateFailedUnknownClass contentGateFailure = "unknown_class"
)

// span reports the slug this failure is reported under. A reader greps one
// prefix for every gate failure and one full name for a single kind.
func (f contentGateFailure) span() string {
	if f == contentGateHealthy {
		return "content.gate.failed"
	}
	return "content.gate.failed." + string(f)
}

// exception maps the failure onto the typed catalog, which is what keeps
// dynamic runtime data out of the exception fields.
func (f contentGateFailure) exception() exceptionCode {
	switch f {
	case contentGateFailedModel:
		return exceptionContentGateModelFailed
	case contentGateFailedUnknownClass:
		return exceptionContentGateUnknownClass
	default:
		return exceptionUnclassified
	}
}

// contentVerdict is what the gate decided about one turn.
type contentVerdict struct {
	// Classified is false when the gate did not run, which is the ordinary
	// case and the reason the tag is worth carrying.
	Classified bool
	Class      ContentClass
	Blocked    bool
}

// parseContentClasses reads the classifier's reply. Ids only, comma separated,
// is the contract; anything else is the classifier failing rather than denying.
func parseContentClasses(reply string) []string {
	fields := strings.FieldsFunc(strings.ToLower(reply), func(r rune) bool {
		return r == ',' || r == '\n' || r == ' ' || r == '\t' || r == '`'
	})
	ids := make([]string, 0, len(fields))
	for _, field := range fields {
		if trimmed := strings.Trim(field, ".:-"); trimmed != "" {
			ids = append(ids, trimmed)
		}
	}
	return ids
}

// recordContentVerdict carries the four tags that were asked for. They are set
// together so a trace never shows a class without the decision about it.
func recordContentVerdict(span trace.Span, verdict contentVerdict) {
	span.SetAttributes(attribute.Bool("content.classified", verdict.Classified))
	if !verdict.Classified {
		return
	}
	class := verdict.Class.ID
	if class == "" {
		class = ContentClassOther
	}
	span.SetAttributes(
		attribute.String("content.class", class),
		attribute.Bool("content.approved", !verdict.Blocked),
		attribute.Bool("content.sensitive", verdict.Class.Sensitive),
	)
}

// classifyTurn asks the taxonomy about a request. A classifier that fails is
// not a denial. See docs/sirens-echo-content-gate.md.
func (a *Agent) classifyTurn(
	ctx context.Context,
	current TranscriptEntry,
	requestID string,
) (contentVerdict, contentGateFailure, error) {
	if len(a.taxonomy.Classes) == 0 {
		return contentVerdict{}, contentGateHealthy, nil
	}
	result, err := a.completions.Complete(ctx, TurnPrompt{
		System:  ContentClassifierPrompt(a.taxonomy),
		Message: current.Content,
	}, requestID)
	if err != nil {
		return contentVerdict{}, contentGateFailedModel, fmt.Errorf("content classifier: %w", err)
	}
	class, blocked, err := a.taxonomy.Verdict(parseContentClasses(result.Content))
	if err != nil {
		return contentVerdict{}, contentGateFailedUnknownClass, fmt.Errorf("content classifier: %w", err)
	}
	return contentVerdict{Classified: true, Class: class, Blocked: blocked}, contentGateHealthy, nil
}

// recordContentGateFailure emits the failure's own span, named for the kind so
// the slug alone separates a dead classifier from one answering off its list.
func (a *Agent) recordContentGateFailure(
	ctx context.Context,
	failure contentGateFailure,
	cause error,
) {
	_, span := a.telemetry.StartSpan(ctx, failure.span())
	a.telemetry.MarkSpanError(span, failure.exception())
	span.End()
	// The error text can quote the class the model invented, so it stays in the
	// log and out of the span, which is the identifier-free surface.
	a.telemetry.Info(
		ctx,
		"content.gate.failed",
		slog.String("failure", string(failure)),
		slog.String("error", cause.Error()),
	)
}
