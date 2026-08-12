package community

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"testing"
	"time"
)

type failingCompletionClient struct{ err error }

func (f failingCompletionClient) Complete(
	context.Context,
	TurnPrompt,
	string,
) (CompletionResult, error) {
	return CompletionResult{}, f.err
}

// deadlineAwareTurn refuses a reply once its context is done, the way a
// transport that respects cancellation does.
type deadlineAwareTurn struct {
	httpTurn
	attempts int
}

func (t *deadlineAwareTurn) Reply(ctx context.Context, content string) error {
	t.attempts++
	if err := ctx.Err(); err != nil {
		return err
	}
	return t.httpTurn.Reply(ctx, content)
}

func failingAgent(cause error) *Agent {
	agent := &Agent{
		cfg:          Config{Definition: Definition{MaxContextMessages: 12}},
		completions:  failingCompletionClient{err: cause},
		systemPrompt: "neutral model policy and local knowledge",
		telemetry:    telemetryOrNoop(nil),
		slots:        make(chan struct{}, 1),
	}
	agent.ensureRuntimeDefaults()
	return agent
}

func TestTurnFailureNoticeNamesTheClass(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name  string
		stage string
		cause error
		want  string
	}{
		{"history", stageHistory, errors.New("channel read"), noticeHistoryFailed},
		{"model", stageModel, errors.New("upstream 502"), noticeModelFailed},
		{"validation", stageValidation, errors.New("ungrounded"), noticeReplyBlocked},
		{"unknown stage", "surprise", errors.New("boom"), noticeTurnFailed},
		{
			"tool surface",
			stageModel,
			ToolFailure{Server: "forgejo", Tool: "create_issue", Err: errors.New("hang")},
			noticeToolFailed,
		},
		{
			"wrapped tool surface",
			stageModel,
			fmt.Errorf("model: %w", ToolFailure{Server: "steam", Err: errors.New("400")}),
			noticeToolFailed,
		},
		{
			"deadline outranks the stage",
			stageModel,
			fmt.Errorf("Agent Proxy request: %w", context.DeadlineExceeded),
			noticeTimedOut,
		},
	}
	for _, testCase := range cases {
		t.Run(testCase.name, func(t *testing.T) {
			t.Parallel()
			got := turnFailureNotice(testCase.stage, testCase.cause)
			if got != testCase.want {
				t.Errorf("notice = %q, want %q", got, testCase.want)
			}
			if !noticeShape.MatchString(got) {
				t.Errorf("notice %q does not match the harness shape", got)
			}
		})
	}
}

// A failed turn used to end as silence, which a member cannot tell apart from
// being ignored. See coilyco-gaming/sirens-echo#138.
func TestFailedTurnRepliesInsteadOfGoingSilent(t *testing.T) {
	t.Parallel()
	agent := failingAgent(errors.New("Agent Proxy returned HTTP 502"))
	turn := &httpTurn{requestID: "failing-turn", current: TranscriptEntry{
		Author:  "member",
		Content: "can you create a fj issue",
	}}

	err := agent.runTurn(context.Background(), turn, nil)

	if err == nil {
		t.Fatal("runTurn returned no error for a failed model call")
	}
	if turn.reply != noticeModelFailed {
		t.Errorf("reply = %q, want %q", turn.reply, noticeModelFailed)
	}
}

// The notice must not inherit the deadline that killed the turn, or the only
// failure a member never hears about is the one that took longest.
func TestFailureNoticeSurvivesAnExpiredTurnContext(t *testing.T) {
	t.Parallel()
	agent := failingAgent(context.DeadlineExceeded)
	expired, cancel := context.WithDeadline(context.Background(), time.Now().Add(-time.Second))
	defer cancel()
	turn := &deadlineAwareTurn{httpTurn: httpTurn{requestID: "expired-turn"}}

	err := agent.failTurn(expired, turn, stageModel, context.DeadlineExceeded)

	if turn.attempts != 1 {
		t.Fatalf("reply attempts = %d, want 1", turn.attempts)
	}
	if turn.reply != noticeTimedOut {
		t.Errorf("reply = %q, want %q", turn.reply, noticeTimedOut)
	}
	if !errors.Is(err, context.DeadlineExceeded) {
		t.Errorf("failTurn error = %v, want the cause preserved", err)
	}
}

// A tool surface that fails is not the model failing, and the member's next
// move differs, so the two do not share a notice.
func TestToolSurfaceFailureIsReportedAsAToolFailure(t *testing.T) {
	t.Parallel()
	agent := failingAgent(ToolFailure{
		Server: "forgejo",
		Tool:   "create_issue",
		Err:    errors.New("context deadline exceeded after 180s"),
	})
	turn := &httpTurn{requestID: "tool-turn"}

	if err := agent.runTurn(context.Background(), turn, nil); err == nil {
		t.Fatal("runTurn returned no error for a failed tool call")
	}
	if turn.reply != noticeToolFailed {
		t.Errorf("reply = %q, want %q", turn.reply, noticeToolFailed)
	}
}

// The notice is written by the harness, so nothing about the cause reaches the
// member. The upstream text stays in telemetry.
func TestFailureNoticeLeaksNothingFromTheCause(t *testing.T) {
	t.Parallel()
	secret := "http://sirens-deep-forgejo-mcp:8080/mcp refused token abc123"
	agent := failingAgent(errors.New(secret))
	turn := &httpTurn{requestID: "leak-turn"}

	if err := agent.runTurn(context.Background(), turn, nil); err == nil {
		t.Fatal("runTurn returned no error")
	}
	for _, fragment := range []string{"sirens-deep", "8080", "abc123", "refused"} {
		if strings.Contains(turn.reply, fragment) {
			t.Errorf("reply %q leaked %q", turn.reply, fragment)
		}
	}
}
