package community

import (
	"errors"
	"fmt"
	"strings"
	"testing"
	"time"
)

func TestHarnessNoticeRendersTheFixedShape(t *testing.T) {
	t.Parallel()
	got := harnessNotice("http 500 internal server error")
	want := "> `http 500 internal server error`"
	if got != want {
		t.Errorf("notice = %q, want %q", got, want)
	}
}

func TestHarnessNoticeSanitizesThePhrase(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name   string
		phrase string
		want   string
	}{
		{"uppercase", "HTTP 404 Not Found", "> `http 404 not found`"},
		{"backtick", "eco tool `not` available", "> `eco tool not available`"},
		{"newline", "rate limit\nexceeded", "> `rate limit exceeded`"},
		{"markdown", "**model** _failed_", "> `model failed`"},
		{"padding", "  turn failed.  ", "> `turn failed`"},
		{"empty", "", "> `" + noticeFallback + "`"},
		{"symbols only", "!!! ***", "> `" + noticeFallback + "`"},
	}
	for _, testCase := range cases {
		t.Run(testCase.name, func(t *testing.T) {
			t.Parallel()
			if got := harnessNotice(testCase.phrase); got != testCase.want {
				t.Errorf("notice = %q, want %q", got, testCase.want)
			}
		})
	}
}

// Every notice the service can emit has to match the shape, because a member
// tells a harness message from a model reply by that shape alone.
func TestEveryHarnessNoticeMatchesTheShape(t *testing.T) {
	t.Parallel()
	notices := map[string]string{
		"turn failed":     noticeTurnFailed,
		"queue timeout":   noticeQueueTimeout,
		"rate limited":    noticeRateLimited,
		"cooldown short":  cooldownNotice(500 * time.Millisecond),
		"cooldown known":  cooldownNotice(90 * time.Second),
		"cooldown minute": cooldownNotice(2 * time.Minute),
	}
	for name, notice := range notices {
		if !noticeShape.MatchString(notice) {
			t.Errorf("%s notice %q does not match the harness shape", name, notice)
		}
		if strings.Count(notice, "`") != 2 {
			t.Errorf("%s notice %q does not carry exactly one code span", name, notice)
		}
	}
}

func TestCooldownNoticeNamesTheWaitWhenItIsKnown(t *testing.T) {
	t.Parallel()
	if got := cooldownNotice(90 * time.Second); got != "> `rate limit exceeded, retry in 1m30s`" {
		t.Errorf("cooldown = %q", got)
	}
	if got := cooldownNotice(200 * time.Millisecond); got != noticeRateLimited {
		t.Errorf("sub-second cooldown = %q, want the plain form", got)
	}
}

// The live incident on issue 258: nine model calls returned 200 and the member
// was told the backend was down, which sent an operator to the wrong system.
func TestExhaustedRoundsDoNotReadAsAnOutage(t *testing.T) {
	t.Parallel()
	cause := fmt.Errorf("Agent Proxy exceeded 6 MCP tool rounds: %w", ErrToolRoundsExhausted)
	got := turnFailureNotice(stageModel, cause)
	if got == noticeModelFailed {
		t.Fatal("an exhausted budget still claims the backend is unavailable")
	}
	if got != noticeRoundsSpent {
		t.Fatalf("notice = %q, want the rounds-spent notice", got)
	}
	// A genuine model-stage failure must keep its own notice.
	if turnFailureNotice(stageModel, errors.New("dial tcp: connection refused")) != noticeModelFailed {
		t.Fatal("an ordinary model failure stopped reading as one")
	}
	// Boundary responses stay short. See issue 175.
	if words := countWords(noticeRoundsSpent); words > 12 {
		t.Fatalf("the notice runs to %d words, which is a handle to pull", words)
	}
}

// Spending the model-call budget is not a backend outage. Every call returned
// 200 and the member was told the backend was down. See issue 258.
func TestSpentBudgetIsNotReportedAsABackendOutage(t *testing.T) {
	t.Parallel()
	spent := fmt.Errorf("Agent Proxy spent 10 model calls: %w", ErrToolRoundsExhausted)
	got := turnFailureNotice(stageModel, spent)
	if got == noticeModelFailed {
		t.Fatal("a spent budget was reported as a backend outage")
	}
	if got != noticeRoundsSpent {
		t.Fatalf("notice = %q, want the rounds-spent notice", got)
	}
}

// A genuine model failure still says so, or the fix would trade one wrong
// notice for another.
func TestARealModelFailureStillSaysBackend(t *testing.T) {
	t.Parallel()
	if got := turnFailureNotice(stageModel, errors.New("connection refused")); got != noticeModelFailed {
		t.Fatalf("notice = %q, want the backend notice", got)
	}
}
