package community

import (
	"context"
	"fmt"
	"strings"

	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/trace"
)

// The gate that makes deny mean something. See docs/sirens-echo-content-gate.md.

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
) (contentVerdict, error) {
	if len(a.taxonomy.Classes) == 0 {
		return contentVerdict{}, nil
	}
	result, err := a.completions.Complete(ctx, TurnPrompt{
		System:  ContentClassifierPrompt(a.taxonomy),
		Message: current.Content,
	}, requestID)
	if err != nil {
		return contentVerdict{}, fmt.Errorf("content classifier: %w", err)
	}
	class, blocked, err := a.taxonomy.Verdict(parseContentClasses(result.Content))
	if err != nil {
		return contentVerdict{}, fmt.Errorf("content classifier: %w", err)
	}
	return contentVerdict{Classified: true, Class: class, Blocked: blocked}, nil
}
