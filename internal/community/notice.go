package community

import (
	"context"
	"errors"
	"fmt"
	"regexp"
	"strings"
	"time"
)

// A notice is a string this service wrote, never a model reply. The rendered
// shape is fixed so a member can tell the two apart. See docs/sirens-echo-notices.md.

const (
	noticeOpen  = "> `"
	noticeClose = "`"
)

// noticeAllowed is the phrase alphabet. Backticks, newlines, and markdown
// cannot survive it, so a notice always renders as one code span.
var noticeAllowed = regexp.MustCompile(`[^a-z0-9 ,./-]+`)

// noticeShape is what every rendered notice matches. The tests assert it
// against each constructor rather than against a copy of the literal.
var noticeShape = regexp.MustCompile("^> `[a-z0-9][a-z0-9 ,./-]*`$")

// noticeFallback stands in for a phrase that sanitizes down to nothing, so a
// caller bug still reaches the member as a notice rather than an empty line.
const noticeFallback = "unspecified harness error"

// harnessNotice renders one short technical phrase in the harness shape. The
// phrase is stated the way a status line is, not the way a sentence is.
func harnessNotice(phrase string) string {
	clean := noticeAllowed.ReplaceAllString(strings.ToLower(phrase), " ")
	clean = strings.Trim(strings.Join(strings.Fields(clean), " "), " ,./-")
	if clean == "" {
		clean = noticeFallback
	}
	return noticeOpen + clean + noticeClose
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
	noticeToolFailed    = harnessNotice("tool call failed")
	noticeRoundsSpent   = harnessNotice("ran out of steps, ask for something narrower")
	noticeReplyBlocked  = harnessNotice("reply blocked by response check, rephrase")
	noticeTurnCrashed   = harnessNotice("turn crashed")
)

// turnFailureNotice names the class a member can act on. The next useful move
// differs per class, which is the whole reason these are not one string.
func turnFailureNotice(stage string, cause error) string {
	switch {
	case errors.Is(cause, context.DeadlineExceeded):
		return noticeTimedOut
	case isToolFailure(cause):
		return noticeToolFailed
	// Ahead of the stage switch, because this happens at the model stage and
	// the backend answered every call. See issue 258.
	case errors.Is(cause, ErrToolRoundsExhausted):
		return noticeRoundsSpent
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
