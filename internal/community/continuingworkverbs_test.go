package community

import (
	"regexp"
	"strings"
	"testing"
)

// Two verbs were in the pattern and could not match anything, because a raw
// string literal made their escape a literal backslash. See sirens-echo#344.

// verbGroupPattern lifts the trailing alternation out of the check, so this
// test reads the live pattern rather than a copy that can rot beside it.
var verbGroupPattern = regexp.MustCompile(`\(\?:([^()]+)\)\\b$`)

func continuingWorkVerbs(t *testing.T) []string {
	t.Helper()
	match := verbGroupPattern.FindStringSubmatch(continuingWorkClaimPattern)
	if match == nil {
		t.Fatalf("the verb alternation is no longer the last group: %s", continuingWorkClaimPattern)
	}
	verbs := strings.Split(match[1], "|")
	if len(verbs) < 2 {
		t.Fatalf("found %d verbs, which cannot be right", len(verbs))
	}
	return verbs
}

// Every alternative has to be reachable. A verb nobody can trigger is worse
// than an absent one, because the list reads as coverage that is not there.
func TestEveryContinuingWorkVerbCanActuallyMatch(t *testing.T) {
	t.Parallel()
	for _, verb := range continuingWorkVerbs(t) {
		t.Run(verb, func(t *testing.T) {
			t.Parallel()
			// The pattern spells a multi-word verb with \s+. A reply spells it
			// with a space, which is what this substitution stands in for.
			phrase := strings.ReplaceAll(verb, `\s+`, " ")
			reply := "Sirens Echo is now " + phrase + " the tracker."
			if !continuingWorkClaim.MatchString(reply) {
				t.Errorf("the verb %q is in the pattern and cannot match %q", verb, reply)
			}
		})
	}
}

// The two phrasings from the issue, kept as themselves rather than generated,
// because they are the observed defect and not an example of it.
func TestTheLookupPhrasingsAreCaught(t *testing.T) {
	t.Parallel()
	for _, reply := range []string{
		"Sirens Echo will look up the issue.",
		"Sirens Echo is now looking up the issue.",
	} {
		if !continuingWorkClaim.MatchString(reply) {
			t.Errorf("not caught: %q", reply)
		}
	}
}

// Widening this pattern is where it has produced false positives before, so
// the must-not-fire side is checked in the same commit as the widening.
func TestTheLookupVerbsDoNotCatchACorrectReply(t *testing.T) {
	t.Parallel()
	for _, reply := range []string{
		"Sirens Echo cannot look up the issue, since no tracker tool is served.",
		"No lookup happened in this turn.",
		"A member can look up the issue on the tracker.",
		"Sirens Echo looked up nothing.",
		"The Eco application will look up prices, not this service.",
	} {
		if continuingWorkClaim.MatchString(reply) {
			t.Errorf("false positive on a correct reply: %q", reply)
		}
	}
}

// A backslash in the pattern is either an escape the regex engine understands
// or the defect this issue was. Nothing here needs a literal one.
func TestThePatternCarriesNoLiteralBackslash(t *testing.T) {
	t.Parallel()
	if strings.Contains(continuingWorkClaimPattern, `\\`) {
		t.Error("a doubled backslash is back in the pattern, which a raw string makes literal")
	}
}
