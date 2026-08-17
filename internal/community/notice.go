package community

import (
	"context"
	"errors"
	"fmt"
	"regexp"
	"strings"
	"time"

	"go.opentelemetry.io/otel/trace"
)

// A notice is a string this service wrote, never a model reply. The rendered
// shape is fixed so a member can tell the two apart. See docs/sirens-echo-delivery.md.

const (
	noticeOpen  = "> `"
	noticeClose = "`"
)

// noticeAllowed is the phrase alphabet, underscore included for tool names.
// See docs/sirens-echo-delivery.md and docs/sirens-echo-worklog.md.
var noticeAllowed = regexp.MustCompile(`[^a-z0-9 ,./_-]+`)

// noticeShape is what every rendered notice matches. A non-ASCII icon may
// lead it, so model prose still cannot pose as one. See sirens-echo-notices.
var noticeShape = regexp.MustCompile("^> (?:[^\\x00-\\x7F]+ )?`[a-z0-9][a-z0-9 ,./_-]*`$")

// noticeFallback stands in for a phrase that sanitizes down to nothing, so a
// caller bug still reaches the member as a notice rather than an empty line.
const noticeFallback = "unspecified harness error"

// harnessNotice renders one short technical phrase in the harness shape. The
// phrase is stated the way a status line is, not the way a sentence is.
func harnessNotice(phrase string) string {
	return noticeOpen + noticeBody(phrase) + noticeClose
}

// noticeBody is the sanitized phrase without its wrapper, so a decorated
// notice reuses the alphabet rather than reimplementing it.
func noticeBody(phrase string) string {
	clean := noticeAllowed.ReplaceAllString(strings.ToLower(phrase), " ")
	clean = strings.Trim(strings.Join(strings.Fields(clean), " "), " ,./-")
	if clean == "" {
		clean = noticeFallback
	}
	return clean
}

// stageNotice says what is happening, and with trailing set that it is still
// happening. An empty icon renders the plain shape.
func stageNotice(icon, phrase string, trailing bool) string {
	body := noticeBody(phrase)
	if trailing {
		body += "..."
	}
	if strings.TrimSpace(icon) == "" {
		return noticeOpen + body + noticeClose
	}
	return "> " + icon + " `" + body + noticeClose
}

// Every member-facing harness string is one of these. Adding a case means
// adding a phrase here, not a sentence at the call site.
var (
	noticeTurnFailed    = harnessNotice("turn failed")
	noticeQueueTimeout  = harnessNotice("busy, retry shortly")
	noticeRateLimited   = harnessNotice("rate limit exceeded, retry shortly")
	noticeTimedOut      = harnessNotice("turn timed out, retry shortly")
	noticeHistoryFailed = harnessNotice("channel history unavailable")
	noticeModelFailed   = harnessNotice("model backend unavailable, retry shortly")
	// The backend answered, and refused the request this service built. Nothing
	// about it is the member's to fix or worth retrying. See sirens-echo#875.
	noticeModelRejected = harnessNotice("harness built a request the model refused")
	// Distinct from the ceiling notice: nothing arrived at all, so retrying is
	// the right advice where at the ceiling it is not. See sirens-echo#171.
	noticeModelSilent  = harnessNotice("model backend went quiet, retry shortly")
	noticeToolFailed   = harnessNotice("tool call failed")
	noticeRoundsSpent  = harnessNotice("ran out of steps, ask for something narrower")
	noticeBudgetSpent  = harnessNotice("ran out of room to answer, ask for something narrower")
	noticeReplyBlocked = harnessNotice("reply blocked by response check, rephrase")
	noticeTurnCrashed  = harnessNotice("turn crashed")
	// The answer existed and the send failed. A member reading silence cannot
	// tell that from being ignored. See docs/sirens-echo-delivery.md.
	noticeUndelivered = harnessNotice("reply could not be delivered, retry shortly")
	// The turn was cut by a restart rather than by anything it did, so retrying
	// is the right move and nothing about it was wrong. See sirens-echo#597.
	noticeShuttingDown = harnessNotice("service restarting, retry shortly")
)

// noticeWithTrace appends the turn's trace so a member's screenshot becomes a
// query. See docs/sirens-echo-delivery.md for why it is not a colon.
func noticeWithTrace(ctx context.Context, notice string) string {
	span := trace.SpanContextFromContext(ctx)
	// Outside a span there is nothing to cite, and a blank id reads as a bug.
	if !span.IsValid() {
		return notice
	}
	return notice + "\n" + harnessNotice("trace id "+span.TraceID().String())
}

// The failure causes, a closed set. error_type is the stage, so two conditions
// at one stage collapse into it and neither can be alerted on. See issue 292.
const (
	causeTimeout = "timeout"
	// The backend sent no bytes at all, which the turn ceiling cannot express.
	causeModelSilent = "model_silent"
	// The backend rejected the request as malformed. Separate from every
	// availability cause, because no retry and no fallback can satisfy it.
	causeModelRejected = "model_rejected"
	causeToolFailed    = "tool_failed"
	causeRoundsSpent   = "rounds_spent"
	// Named for what happened rather than where, because the stage is model
	// and the model is not what failed. See sirens-echo#651.
	causeReplyRefused = "reply_refused"
	// The model deliberated past the ceiling and emitted nothing. Separate from
	// rounds_spent because the ceiling that bound it is a different number.
	causeBudgetSpent = "budget_spent"
	causeStage       = "stage_failed"
	// causeShutdown is the service ending the turn, not the turn failing. It
	// reads as a deploy in the failure series rather than as a defect.
	causeShutdown = "shutdown"
)

// failureCause classifies what went wrong, in the same order the notice does,
// so the label and the phrase a member reads can never disagree.
func failureCause(cause error) string {
	switch {
	// Ahead of the timeout, because a drain cancels a turn that still had
	// budget and the reason it ended is the restart.
	case errors.Is(cause, errShuttingDown):
		return causeShutdown
	// Ahead of the timeout, because an idle cut is also a deadline and the
	// interesting half is that nothing arrived.
	case errors.Is(cause, ErrModelSilent):
		return causeModelSilent
	case errors.Is(cause, context.DeadlineExceeded):
		return causeTimeout
	case isToolFailure(cause):
		return causeToolFailed
	case errors.Is(cause, ErrToolRoundsExhausted):
		return causeRoundsSpent
	// The harness rejected the reply. Distinguishing that from a model failure
	// is acceptance criterion 3 of sirens-echo#651.
	case errors.Is(cause, ErrResponseRepairExhausted):
		return causeReplyRefused
	// The model thought past its ceiling. Collapsed into stage_failed, a turn
	// the ceiling bound was unalertable. See sirens-echo#549.
	case errors.Is(cause, ErrBudgetExhausted):
		return causeBudgetSpent
	case rejectedByModel(cause):
		return causeModelRejected
	}
	return causeStage
}

// turnFailureNotice names the class a member can act on. The next useful move
// differs per class, which is the whole reason these are not one string.
func turnFailureNotice(stage string, cause error) string {
	switch {
	// Same order as failureCause, so the label and the phrase agree.
	case errors.Is(cause, errShuttingDown):
		return noticeShuttingDown
	case errors.Is(cause, ErrModelSilent):
		return noticeModelSilent
	case errors.Is(cause, context.DeadlineExceeded):
		return noticeTimedOut
	case isToolFailure(cause):
		return noticeToolFailed
	// Ahead of the stage switch, because this happens at the model stage and
	// the backend answered every call. See issue 258.
	case errors.Is(cause, ErrToolRoundsExhausted):
		return noticeRoundsSpent
	// Every model call returned 200 and the reply was refused here, so the
	// member is told what to do about it rather than that we are down. See #651.
	case errors.Is(cause, ErrResponseRepairExhausted):
		return noticeReplyBlocked
	// The backend answered and the model thought until it had no room to speak.
	// Narrowing the question is the move that works. See sirens-echo#549.
	case errors.Is(cause, ErrBudgetExhausted):
		return noticeBudgetSpent
	// Ahead of the stage switch, because this happens at the model stage and
	// the backend was reachable the whole time. See sirens-echo#875.
	case rejectedByModel(cause):
		return noticeModelRejected
	}
	switch stage {
	case stageHistory:
		return noticeHistoryFailed
	case stageModel:
		return noticeModelFailed
	case stageValidation:
		return noticeReplyBlocked
	}
	return noticeTurnFailed
}

// The turn stages, which are also the failure metric's label values.
const (
	stageHistory    = "history"
	stageModel      = "model"
	stageValidation = "validation"
)

// cooldownNotice names the wait when the limiter knows one. A sub-second retry
// rounds to nothing useful, so it takes the plain form.
func cooldownNotice(retryAfter time.Duration) string {
	if retryAfter < time.Second {
		return noticeRateLimited
	}
	return harnessNotice(fmt.Sprintf(
		"rate limit exceeded, retry in %s",
		retryAfter.Round(time.Second),
	))
}
