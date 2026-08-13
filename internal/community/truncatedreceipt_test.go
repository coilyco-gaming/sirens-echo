package community

import (
	"strings"
	"testing"
)

// The tool result says a page was cut and the receipt does not, so the model
// learns it and the member does not. See sirens-echo#435.

func truncatedFetchResult() string {
	return fetchText(200, []byte(strings.Repeat("a", maxFetchBytes+1)))
}

// Characterization. The fix wants a fourth ToolOutcome for a call that worked
// and returned less than everything, which is a decision rather than a patch.
func TestATruncatedFetchReceiptLooksLikeAWholeOne(t *testing.T) {
	t.Parallel()
	whole := outcomeOf(ToolResult{Text: fetchText(200, []byte("a short page"))})
	cut := outcomeOf(ToolResult{Text: truncatedFetchResult()})
	if toolOutcomeGlyph(cut) != toolOutcomeGlyph(whole) {
		t.Fatalf("a truncated fetch now reads %s against %s for a whole page, so the "+
			"receipt distinguishes them and this test should go with issue 435",
			toolOutcomeGlyph(cut), toolOutcomeGlyph(whole))
	}
}

// The half that does work, pinned beside the half that does not, so a change to
// either is visible in one place.
func TestATruncatedFetchTellsTheModelEvenSo(t *testing.T) {
	t.Parallel()
	if !strings.Contains(truncatedFetchResult(), "truncated at") {
		t.Error("the tool result no longer says the page was cut, which is the half " +
			"of issue 435 that shipped")
	}
	whole := fetchText(200, []byte("a short page"))
	if strings.Contains(whole, "truncated") {
		t.Errorf("a page within the cap was marked as cut: %q", whole)
	}
}
