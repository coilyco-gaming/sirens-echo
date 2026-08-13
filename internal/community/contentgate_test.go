package community

import (
	"context"
	"strings"
	"testing"
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
			{ID: "irl-physical", Summary: "the physical world", Deny: true},
			{ID: "self-harm", Summary: "harm", Deny: true, Sensitive: true},
		},
	}
}

func TestTheClassifierReplyIsReadLeniently(t *testing.T) {
	t.Parallel()
	for _, reply := range []string{
		"irl-physical",
		"irl-physical, other",
		"`irl-physical`,other\n",
		"IRL-PHYSICAL",
		" irl-physical .",
	} {
		class, blocked, err := gateTaxonomy().Verdict(parseContentClasses(reply))
		if err != nil {
			t.Errorf("reply %q: %v", reply, err)
			continue
		}
		if !blocked || class.ID != "irl-physical" {
			t.Errorf("reply %q gave class %q blocked=%v", reply, class.ID, blocked)
		}
	}
}

// Sensitive wins, because a request tripping both must never be refused by the
// shape that names the ordinary category.
func TestSensitiveWinsOverAnOrdinaryDenial(t *testing.T) {
	t.Parallel()
	class, blocked, err := gateTaxonomy().Verdict(parseContentClasses("irl-physical, self-harm"))
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
	verdict, err := agent.classifyTurn(t.Context(), TranscriptEntry{Content: "where is the bus"}, "req-1")
	if err != nil {
		t.Fatalf("classifyTurn: %v", err)
	}
	if verdict.Classified || verdict.Blocked {
		t.Errorf("an unconfigured gate produced a verdict: %+v", verdict)
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
		ContentClass{ID: "irl-physical", Deny: true},
		"",
		PlaceholderPrincipal,
	)
	if strings.TrimSpace(rendered) == "" {
		t.Fatal("a blocked turn produced an empty reply")
	}
}
