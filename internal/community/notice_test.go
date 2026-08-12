package community

import (
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
