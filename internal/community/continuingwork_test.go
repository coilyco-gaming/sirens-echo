package community

import (
	"strings"
	"testing"
)

// The reply path must reject a promise of work between turns, not only the
// deployment gate. See docs/sirens-echo-continuing-work.md.

// The gate and the runtime hold one definition. Two copies drift quietly: the
// gate keeps passing while the runtime stops matching, or the reverse.
func TestContinuingWorkClaimIsPinnedToTheDeploymentGate(t *testing.T) {
	t.Parallel()
	pack, err := LoadEvaluationPack("../../agent/evaluation.yaml")
	if err != nil {
		t.Fatalf("load evaluation pack: %v", err)
	}
	var found *EvaluationCase
	for index, evaluationCase := range pack.Cases {
		if evaluationCase.ID == "no-continuing-work-claim" {
			found = &pack.Cases[index]
			break
		}
	}
	if found == nil {
		t.Fatal("no-continuing-work-claim is gone from the pack; the runtime " +
			"check now has no gate to agree with")
	}
	if len(found.ForbiddenPatterns) != 1 {
		t.Fatalf("case carries %d patterns, want exactly 1 to pin against",
			len(found.ForbiddenPatterns))
	}
	if found.ForbiddenPatterns[0] != continuingWorkClaimPattern {
		t.Errorf("the gate and the runtime have drifted apart.\n gate: %s\n code: %s",
			found.ForbiddenPatterns[0], continuingWorkClaimPattern)
	}
}

func TestValidateGroundingRejectsContinuingWorkClaims(t *testing.T) {
	t.Parallel()
	// The first is Lucia's measured case. The rest are the shapes a member
	// would read as a promise someone is coming back to them.
	for _, reply := range []string{
		"The system is now processing these requests.",
		"Sirens Echo will keep watching the Eco server.",
		"Sirens Echo will notify you when it comes back up.",
		"The service will continue to monitor the queue.",
		"This service is now tracking that for you.",
	} {
		if err := ValidateGrounding(reply, ""); err == nil {
			t.Errorf("shipped a promise the runtime cannot keep: %q", reply)
		}
	}
}

// A reply that correctly refuses must still ship. Blocking one costs the member
// the answer, which is the outcome this check exists to prevent.
func TestValidateGroundingAllowsRefusalsAndOrdinaryReplies(t *testing.T) {
	t.Parallel()
	for _, reply := range []string{
		"Sirens Echo will not keep watching the server.",
		"Sirens Echo cannot monitor the server between messages.",
		"I have no way to watch the server, so ask again later.",
		"The server is up right now.",
		"Ask again in a while and the answer will be current.",
	} {
		if err := ValidateGrounding(reply, ""); err != nil {
			t.Errorf("blocked a reply that should ship: %q (%v)", reply, err)
		}
	}
}

// The runtime pattern is English-only by inheritance, and that is deliberate.
// Recorded so widening it surfaces here rather than as a surprise.
func TestContinuingWorkClaimIsEnglishOnly(t *testing.T) {
	t.Parallel()
	translated := "Sirens Echo va continuer a surveiller le serveur Eco."
	if err := ValidateGrounding(translated, ""); err == nil {
		t.Log("known limitation: the French form is not caught")
	} else {
		t.Errorf("the check gained a language without this test being updated: %v", err)
	}
	if !strings.Contains(continuingWorkClaimPattern, "(?i)") {
		t.Error("pattern lost its case-insensitive flag")
	}
}
