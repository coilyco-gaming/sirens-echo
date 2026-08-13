package community

import "testing"

// outcomeOf reads the envelope: an error, an empty string, or anything else.
// A result saying it searched two percent is anything else. See #449.

// The verbatim shape from sirens-echo#449, which a member received as "none
// exists". It classifies as ok, the same as a call that returned every row.
const partialCoverageResult = "22405 older trades arrive as 3152 hourly " +
	"rollups; party, item, store, and unit-price views cover detailed rows " +
	"only\nNo markets matched item='wooden hull plank' across 528 ledger rows"

// Asserted as it ships. A found-nothing result is indistinguishable from a
// found-everything one at this layer. See sirens-echo#449.
func TestAPartialCoverageResultIsIndistinguishableFromASuccess(t *testing.T) {
	t.Parallel()
	partial := outcomeOf(ToolResult{Text: partialCoverageResult})
	full := outcomeOf(ToolResult{Text: "3 markets matched, 12 rows"})

	if partial != full {
		t.Errorf("a partial-coverage result now reports %q against %q for a "+
			"complete one. If sirens-echo#449 gained a state, assert the new "+
			"one here instead", partial, full)
	}
	if partial != ToolOutcomeOK {
		t.Errorf("partial coverage reports %q, expected ok", partial)
	}
}

// The other two states, so the test above cannot pass by outcomeOf collapsing
// to one value. ToolOutcomeEmpty has never been observed in production.
func TestTheOtherTwoOutcomesStillSeparate(t *testing.T) {
	t.Parallel()
	if got := outcomeOf(ToolResult{IsError: true, Text: "boom"}); got != ToolOutcomeFailed {
		t.Errorf("an errored result reports %q, expected failed", got)
	}
	if got := outcomeOf(ToolResult{Text: "   "}); got != ToolOutcomeEmpty {
		t.Errorf("a blank result reports %q, expected empty", got)
	}
	// Measured over 24h of production calls: ok and failed occur, empty does
	// not. A real tool answers "nothing matched" in prose. See #449.
	if ToolOutcomeEmpty == ToolOutcomeOK {
		t.Error("empty and ok are the same value, so the state above is not a state")
	}
}
