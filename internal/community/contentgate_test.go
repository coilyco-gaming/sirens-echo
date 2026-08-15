package community

import (
	"context"
	"errors"
	"strings"
	"testing"

	"go.opentelemetry.io/otel/codes"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
)

// recordedTags runs record against a real span and returns what it set.
func recordedTags(t *testing.T, verdict contentVerdict) map[string]string {
	t.Helper()
	telemetry, recorder, _ := jobTelemetry(t)
	_, span := telemetry.StartSpan(context.Background(), "community.turn")
	recordContentVerdict(span, verdict)
	span.End()
	ended := recorder.Ended()
	if len(ended) != 1 {
		t.Fatalf("recorded %d spans", len(ended))
	}
	// Emit rather than AsString, because three of the four tags are booleans.
	tags := map[string]string{}
	for _, pair := range ended[0].Attributes() {
		tags[string(pair.Key)] = pair.Value.Emit()
	}
	return tags
}

// deny: true was a declaration with nothing to enforce it. See sirens-echo#227
// and sirens-echo#228.

func gateTaxonomy() ContentTaxonomy {
	return ContentTaxonomy{
		Schema: ContentClassSchema,
		Classes: []ContentClass{
			{ID: ContentClassOther, Summary: "anything else"},
			{ID: "eco-server", Summary: "the Eco server"},
			{ID: "medical-legal-advice", Summary: "diagnosis or legal guidance", Deny: true},
			{ID: "self-harm", Summary: "harm", Deny: true, Sensitive: true},
		},
	}
}

func TestTheClassifierReplyIsReadLeniently(t *testing.T) {
	t.Parallel()
	for _, reply := range []string{
		"medical-legal-advice",
		"medical-legal-advice, other",
		"`medical-legal-advice`,other\n",
		"MEDICAL-LEGAL-ADVICE",
		" medical-legal-advice .",
	} {
		class, blocked, err := gateTaxonomy().Verdict(parseContentClasses(reply))
		if err != nil {
			t.Errorf("reply %q: %v", reply, err)
			continue
		}
		if !blocked || class.ID != "medical-legal-advice" {
			t.Errorf("reply %q gave class %q blocked=%v", reply, class.ID, blocked)
		}
	}
}

// Sensitive wins, because a request tripping both must never be refused by the
// shape that names the ordinary category.
func TestSensitiveWinsOverAnOrdinaryDenial(t *testing.T) {
	t.Parallel()
	class, blocked, err := gateTaxonomy().Verdict(parseContentClasses("medical-legal-advice, self-harm"))
	if err != nil || !blocked {
		t.Fatalf("blocked=%v err=%v", blocked, err)
	}
	if !class.Sensitive {
		t.Errorf("class = %q, want the sensitive one", class.ID)
	}
}

// An allowed class is not a block, which is the case that must not regress:
// the gate exists to refuse a few things, not to become a refusal machine.
func TestAnAllowedClassPassesThrough(t *testing.T) {
	t.Parallel()
	_, blocked, err := gateTaxonomy().Verdict(parseContentClasses("eco-server"))
	if err != nil {
		t.Fatalf("verdict: %v", err)
	}
	if blocked {
		t.Error("an allowed class was blocked")
	}
}

// A classifier answering with something outside the closed set is the
// classifier failing. The caller treats that as no verdict, not as a denial.
func TestAnUnknownClassIsAnErrorRatherThanADenial(t *testing.T) {
	t.Parallel()
	if _, _, err := gateTaxonomy().Verdict(parseContentClasses("bus-timetable")); err == nil {
		t.Error("an unknown class was accepted")
	}
}

func TestTheFourTagsAreCarried(t *testing.T) {
	t.Parallel()
	tags := recordedTags(t, contentVerdict{
		Classified: true,
		Class:      ContentClass{ID: "self-harm", Deny: true, Sensitive: true},
		Blocked:    true,
	})
	for key, want := range map[string]string{
		"content.classified": "true",
		"content.class":      "self-harm",
		"content.approved":   "false",
		"content.sensitive":  "true",
	} {
		if tags[key] != want {
			t.Errorf("%s = %q, want %q", key, tags[key], want)
		}
	}
}

// The classified tag earns its place by being false sometimes. A turn the gate
// never ran on must not report a class it never decided.
func TestAnUnclassifiedTurnCarriesNoClass(t *testing.T) {
	t.Parallel()
	tags := recordedTags(t, contentVerdict{})
	if tags["content.classified"] != "false" {
		t.Errorf("content.classified = %q, want false", tags["content.classified"])
	}
	for _, absent := range []string{"content.class", "content.approved", "content.sensitive"} {
		if _, present := tags[absent]; present {
			t.Errorf("%s was set on a turn that was never classified", absent)
		}
	}
}

// An empty taxonomy is the default, and it must cost nothing: no model call,
// no verdict, no tag beyond the false one.
func TestNoTaxonomyRunsNoClassifier(t *testing.T) {
	t.Parallel()
	agent := &Agent{completions: refusingCompletions{t: t}}
	verdict, failure, err := agent.classifyTurn(t.Context(), TranscriptEntry{Content: "where is the bus"}, "req-1")
	if err != nil {
		t.Fatalf("classifyTurn: %v", err)
	}
	if verdict.Classified || verdict.Blocked {
		t.Errorf("an unconfigured gate produced a verdict: %+v", verdict)
	}
	if failure != contentGateHealthy {
		t.Errorf("an unconfigured gate reported failure %q, want none", failure)
	}
}

// refusingCompletions fails the test if it is ever called.
type refusingCompletions struct{ t *testing.T }

func (c refusingCompletions) Complete(
	_ context.Context, _ TurnPrompt, _ string,
) (CompletionResult, error) {
	c.t.Error("the classifier ran with no taxonomy configured")
	return CompletionResult{}, nil
}

// A denied class still has to produce a refusal a member can read.
func TestABlockedClassProducesAReadableRefusal(t *testing.T) {
	t.Parallel()
	rendered := BlockResponse(
		ContentClass{ID: "medical-legal-advice", Deny: true},
		"",
		PlaceholderPrincipal,
	)
	if strings.TrimSpace(rendered) == "" {
		t.Fatal("a blocked turn produced an empty reply")
	}
}

// failingCompletions makes the classifier call fail.
type failingCompletions struct{ err error }

func (c failingCompletions) Complete(
	_ context.Context, _ TurnPrompt, _ string,
) (CompletionResult, error) {
	return CompletionResult{}, c.err
}

// answeringCompletions replies with whatever the test wants classified.
type answeringCompletions struct{ reply string }

func (c answeringCompletions) Complete(
	_ context.Context, _ TurnPrompt, _ string,
) (CompletionResult, error) {
	return CompletionResult{Content: c.reply}, nil
}

// gateFailureSpan runs one failing classification and returns the span the
// failure emitted, plus the log line beside it.
func gateFailureSpan(t *testing.T, completions CompletionClient) (sdktrace.ReadOnlySpan, string) {
	t.Helper()
	telemetry, recorder, logs := jobTelemetry(t)
	agent := &Agent{telemetry: telemetry, completions: completions, taxonomy: gateTaxonomy()}
	verdict, failure, err := agent.classifyTurn(
		t.Context(), TranscriptEntry{Content: "where is the bus"}, "req-1",
	)
	if err == nil {
		t.Fatal("the classifier did not fail")
	}
	if verdict.Classified || verdict.Blocked {
		// The property the whole gate rests on, asserted where it can break.
		t.Fatalf("a broken gate produced a verdict: %+v", verdict)
	}
	agent.recordContentGateFailure(t.Context(), failure, err)
	ended := recorder.Ended()
	if len(ended) != 1 {
		t.Fatalf("recorded %d spans, want 1", len(ended))
	}
	return ended[0], logs.String()
}

// The ask: the slug alone says which way the gate broke, so a dead classifier
// and one answering off its list are separable without opening a span.
func TestAGateFailureNamesItsKindInTheSlug(t *testing.T) {
	t.Parallel()
	for _, testCase := range []struct {
		name        string
		completions CompletionClient
		want        string
		outcome     string
	}{
		{
			name:        "the model call fails",
			completions: failingCompletions{err: errors.New("upstream refused")},
			want:        "content.gate.failed.model",
			outcome:     "model_failed",
		},
		{
			name:        "the model answers off its list",
			completions: answeringCompletions{reply: "bus-timetable"},
			want:        "content.gate.failed.unknown_class",
			outcome:     "unknown_class",
		},
	} {
		t.Run(testCase.name, func(t *testing.T) {
			t.Parallel()
			span, _ := gateFailureSpan(t, testCase.completions)
			if got := span.Name(); got != testCase.want {
				t.Errorf("span name = %q, want %q", got, testCase.want)
			}
			if got := span.Status().Code; got != codes.Error {
				t.Errorf("status = %v, want an error", got)
			}
			attributes := stringAttributes(span.Attributes())
			if got := attributes["error.outcome"]; got != testCase.outcome {
				t.Errorf("error.outcome = %q, want %q", got, testCase.outcome)
			}
			if got := attributes["error.stage"]; got != "content_gate" {
				t.Errorf("error.stage = %q, want content_gate", got)
			}
		})
	}
}

// The slug is a closed vocabulary, so the class the model invented reaches the
// log and never the span. A name carrying model output is unbounded.
func TestAGateFailureSpanCarriesNoModelOutput(t *testing.T) {
	t.Parallel()
	span, logged := gateFailureSpan(t, answeringCompletions{reply: "bus-timetable"})
	if strings.Contains(span.Name(), "bus-timetable") {
		t.Errorf("span name %q carries the invented class", span.Name())
	}
	for _, pair := range span.Attributes() {
		if strings.Contains(pair.Value.Emit(), "bus-timetable") {
			t.Errorf("span attribute %s carries the invented class", pair.Key)
		}
	}
	if !strings.Contains(logged, "bus-timetable") {
		t.Error("the invented class reached neither the span nor the log, so it is unrecoverable")
	}
}

// Every failure kind has a cataloged exception. A kind added without one would
// report as unclassified while still looking specific in the slug.
func TestEveryGateFailureKindIsCataloged(t *testing.T) {
	t.Parallel()
	for _, failure := range []contentGateFailure{contentGateFailedModel, contentGateFailedUnknownClass} {
		if failure.exception() == exceptionUnclassified {
			t.Errorf("failure %q maps to the unclassified exception", failure)
		}
		if got := exceptionFor(failure.exception()).stage; got != "content_gate" {
			t.Errorf("failure %q has stage %q, want content_gate", failure, got)
		}
	}
}
