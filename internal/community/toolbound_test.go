package community

import (
	"strconv"
	"strings"
	"testing"
)

// The span assertions use the recorder harness in tooloutcomespan_test.go.

// A truncation was decided after the tool span closed, so it could reach a log
// and never the trace. See sirens-echo#640.

// The delivered count is the point of the signature. Subtracting the limit
// reports the loss wrongly whenever the walk-back cuts below it.
func TestTheDeliveredCountIsNotAlwaysTheLimit(t *testing.T) {
	t.Parallel()
	// A three-byte rune straddling the cap forces the walk-back.
	huge := strings.Repeat("あ", 4096)
	limit := 1000

	bounded, delivered, trimmed := boundToolResult(huge, limit)

	if !trimmed {
		t.Fatal("a result past the cap was not trimmed")
	}
	if delivered >= limit {
		t.Errorf("delivered = %d, want the walk-back to cut below %d", delivered, limit)
	}
	if delivered%3 != 0 {
		t.Errorf("delivered = %d, want a whole number of runes", delivered)
	}
	// The naive subtraction is what the record used to imply, and it is wrong
	// here by exactly the walk-back.
	if len(huge)-delivered == len(huge)-limit {
		t.Error("the real loss and the naive one agree, so this case proves nothing")
	}
	if !strings.Contains(bounded, "truncated by the runtime") {
		t.Error("the bounded result carries no truncation notice")
	}
}

// A result inside the cap delivers all of it and drops nothing, so the ordinary
// tool call reports a zero loss rather than a negative one.
func TestAResultInsideTheCapDeliversAllOfIt(t *testing.T) {
	t.Parallel()
	small := "twelve rows"

	bounded, delivered, trimmed := boundToolResult(small, maxToolResultBytes)

	if trimmed {
		t.Error("a small result was trimmed")
	}
	if bounded != small {
		t.Errorf("bounded = %q, want it untouched", bounded)
	}
	if delivered != len(small) {
		t.Errorf("delivered = %d, want %d", delivered, len(small))
	}
	if dropped := len(small) - delivered; dropped != 0 {
		t.Errorf("dropped = %d, want nothing lost", dropped)
	}
}

// An ASCII result cuts exactly at the limit, which is the case where naive
// subtraction happens to be right and must stay right.
func TestAnAsciiResultCutsAtTheLimit(t *testing.T) {
	t.Parallel()
	huge := strings.Repeat("a", 4096)
	limit := 1000

	_, delivered, trimmed := boundToolResult(huge, limit)

	if !trimmed {
		t.Fatal("a result past the cap was not trimmed")
	}
	if delivered != limit {
		t.Errorf("delivered = %d, want exactly the limit %d", delivered, limit)
	}
}

// The headline. A truncation reaches the trace, which it could not while the
// span was closed before the bound was applied.
func TestTheTraceSaysWhetherAResultWasTruncated(t *testing.T) {
	t.Parallel()
	span := toolCallSpan(t, FixtureTool{
		Name:   "find_trade",
		Server: "eco",
		Result: strings.Repeat("row\n", maxToolResultBytes/2),
	})

	if got := recordedAttribute(span, "mcp.tool.truncated"); got != "true" {
		t.Errorf("mcp.tool.truncated = %q, want true", got)
	}
	want := strconv.Itoa(maxToolResultBytes)
	if got := recordedAttribute(span, "mcp.tool.limit_bytes"); got != want {
		t.Errorf("mcp.tool.limit_bytes = %q, want the packaged %s", got, want)
	}
	// The bound and the bytes that arrived are different numbers, which is the
	// confusion the attribute exists to end.
	if got := recordedAttribute(span, "mcp.tool.result_bytes"); got == want {
		t.Error("result_bytes and limit_bytes agree, so the case is not truncating")
	}
}

// A result inside the cap says so on the trace, so absence of the attribute is
// never what a reader has to infer from.
func TestAnUntruncatedResultSaysSoOnTheTrace(t *testing.T) {
	t.Parallel()
	span := toolCallSpan(t, FixtureTool{
		Name: "find_trade", Server: "eco", Result: "913 offers",
	})

	if got := recordedAttribute(span, "mcp.tool.truncated"); got != "false" {
		t.Errorf("mcp.tool.truncated = %q, want false", got)
	}
	if want := strconv.Itoa(maxToolResultBytes); recordedAttribute(span, "mcp.tool.limit_bytes") != want {
		t.Errorf("mcp.tool.limit_bytes = %q, want the bound %s named either way",
			recordedAttribute(span, "mcp.tool.limit_bytes"), want)
	}
}
