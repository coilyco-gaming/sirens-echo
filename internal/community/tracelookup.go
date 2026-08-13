package community

import (
	"context"
	"log/slog"
	"regexp"
	"strings"
)

// Recognising that a member is asking about a trace. Fetching the trace is a
// separate half. See docs/sirens-echo-trace-lookup.md.

// traceIDPattern is the W3C trace id: 32 lowercase hex characters. Anchored to
// word boundaries so a longer hash cannot be read as one.
var traceIDPattern = regexp.MustCompile(`\b[0-9a-f]{32}\b`)

// traceKeywordPattern requires the word itself, not a substring of another.
// Without it, any message quoting a hex id would become a lookup.
var traceKeywordPattern = regexp.MustCompile(`(?i)\btraces?\b`)

// traceLookup is what a member asked for, resolved from what they typed and
// from the message they replied to.
type traceLookup struct {
	TraceID string
	// Quoted reports that the id came from the replied-to message rather than
	// from the member's own text, which is the common case.
	Quoted bool
}

// detectTraceLookup reads a lookup out of a turn. Both halves are required: the
// word asks for it, the id says which one. See docs/sirens-echo-trace-lookup.md.
func detectTraceLookup(message, referenced string) (traceLookup, bool) {
	if !traceKeywordPattern.MatchString(message) {
		return traceLookup{}, false
	}
	if id := traceIDPattern.FindString(strings.ToLower(message)); id != "" {
		return traceLookup{TraceID: id}, true
	}
	// A member who replies to a notice and says "trace" has named the id by
	// pointing at it, which is the shape this feature exists for.
	if id := traceIDPattern.FindString(strings.ToLower(referenced)); id != "" {
		return traceLookup{TraceID: id, Quoted: true}, true
	}
	return traceLookup{}, false
}

// traceRequester is an optional turn capability, asserted like the reactor is.
// Only a transport that can carry a reply reference implements it.
type traceRequester interface {
	TraceLookup() (traceLookup, bool)
}

// recordTraceLookup counts the ask. The retrieval half needs a tracing backend
// this service has no grant for, so today this is the whole of the feature.
func (a *Agent) recordTraceLookup(ctx context.Context, turn turnIO) {
	requester, ok := turn.(traceRequester)
	if !ok {
		return
	}
	lookup, asked := requester.TraceLookup()
	if !asked {
		return
	}
	// The id is this service's own identifier and carries no member data, so
	// it is recorded whole rather than counted blind.
	a.telemetry.Info(
		ctx,
		"turn.trace.requested",
		slog.String("trace_id", lookup.TraceID),
		slog.Bool("quoted", lookup.Quoted),
		slog.Bool("served", false),
	)
}
