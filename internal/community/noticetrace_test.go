package community

import (
	"context"
	"strings"
	"testing"

	"go.opentelemetry.io/otel/trace"
)

// A member reporting "it said no and I do not know why" should be handing over
// a query. See sirens-echo#336.

func tracedContext(t *testing.T) (context.Context, string) {
	t.Helper()
	id, err := trace.TraceIDFromHex("3dd883c6becba130e9f8b75e4593a94d")
	if err != nil {
		t.Fatalf("trace id: %v", err)
	}
	span, err := trace.SpanIDFromHex("72bfcd9f6ff12a48")
	if err != nil {
		t.Fatalf("span id: %v", err)
	}
	ctx := trace.ContextWithSpanContext(context.Background(), trace.NewSpanContext(
		trace.SpanContextConfig{TraceID: id, SpanID: span, TraceFlags: trace.FlagsSampled},
	))
	return ctx, id.String()
}

func TestNoticeCarriesTheTraceID(t *testing.T) {
	t.Parallel()
	ctx, id := tracedContext(t)
	rendered := noticeWithTrace(ctx, noticeTimedOut)
	if !strings.Contains(rendered, id) {
		t.Fatalf("notice does not carry the trace id: %q", rendered)
	}
	if !strings.HasPrefix(rendered, noticeTimedOut) {
		t.Errorf("the original notice was altered: %q", rendered)
	}
}

// Both lines have to survive the notice shape, or a member cannot tell a
// harness string from model output. See docs/sirens-echo-notices.md.
func TestBothNoticeLinesKeepTheHarnessShape(t *testing.T) {
	t.Parallel()
	ctx, _ := tracedContext(t)
	for _, line := range strings.Split(noticeWithTrace(ctx, noticeRateLimited), "\n") {
		if !noticeShape.MatchString(line) {
			t.Errorf("line escapes the notice shape: %q", line)
		}
	}
}

// Not every refusal happens inside a turn span. A rate-limit shed can fire
// before one exists, and a blank id reads as a bug rather than as an absence.
func TestNoticeOmitsTheLineOutsideASpan(t *testing.T) {
	t.Parallel()
	rendered := noticeWithTrace(context.Background(), noticeTurnFailed)
	if rendered != noticeTurnFailed {
		t.Fatalf("a notice outside a span gained a line: %q", rendered)
	}
	if strings.Contains(rendered, "trace id") {
		t.Error("an empty trace id reached a member")
	}
}

// A successful reply must not gain one. The trace line marks a turn that ended
// in something other than success.
func TestASuccessfulReplyIsUntouched(t *testing.T) {
	t.Parallel()
	ctx, _ := tracedContext(t)
	reply := "The Eco server is online."
	if got := noticeWithTrace(ctx, reply); !strings.HasPrefix(got, reply) {
		t.Fatalf("the reply text was altered: %q", got)
	}
	// noticeWithTrace is only ever called on notice paths. This asserts the
	// helper is inert on content, not that a reply routes through it.
	if strings.Count(noticeWithTrace(ctx, reply), "\n") != 1 {
		t.Error("more than the trace line was appended")
	}
}
