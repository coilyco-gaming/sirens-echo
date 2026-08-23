package community

import (
	"context"
	"errors"
	"strings"
	"testing"
)

// editableTurn is a transport that can revise what it sent, standing in for
// the Discord one without a session.
type editableTurn struct {
	*httpTurn
	thread   string
	edited   string
	body     string
	failEdit bool
}

func (t *editableTurn) ThreadID() string { return t.thread }

func (t *editableTurn) EditReply(_ context.Context, messageID, content string) error {
	if t.failEdit {
		return errors.New("unknown message")
	}
	t.edited, t.body = messageID, content
	return nil
}

func summaryAgent(t *testing.T, enabled bool) *Agent {
	t.Helper()
	agent := &Agent{
		cfg:          Config{Definition: Definition{MaxContextMessages: 12}, ThreadSummary: enabled},
		systemPrompt: "neutral model policy",
		telemetry:    telemetryOrNoop(nil),
	}
	agent.ensureRuntimeDefaults()
	return agent
}

func summaryTurn(thread string) *editableTurn {
	return &editableTurn{
		httpTurn: &httpTurn{requestID: "turn-1", transport: transportDiscord},
		thread:   thread,
	}
}

// The shape Kai chose for sirens-echo#951: a thread carries one living answer
// rather than a run of replies.
func TestASecondTurnInAThreadRevisesTheFirstReply(t *testing.T) {
	t.Parallel()
	agent := summaryAgent(t, true)
	agent.summaries.Remember("thread-1", "reply-1")
	turn := summaryTurn("thread-1")

	if err := agent.sendReply(context.Background(), turn, "the answer, revised", nothingWithheld); err != nil {
		t.Fatalf("sendReply: %v", err)
	}
	if turn.edited != "reply-1" {
		t.Errorf("edited %q, want the thread's first reply", turn.edited)
	}
	if turn.body != "the answer, revised" {
		t.Errorf("body = %q, want the new content", turn.body)
	}
	if turn.reply != "" {
		t.Errorf("also posted %q, so the thread gained a message", turn.reply)
	}
}

// Off is the shipped default, so an unflagged deployment is unchanged.
func TestTheFeatureIsOffUnlessTheDeploymentAsks(t *testing.T) {
	t.Parallel()
	agent := summaryAgent(t, false)
	agent.summaries.Remember("thread-1", "reply-1")
	turn := summaryTurn("thread-1")

	if err := agent.sendReply(context.Background(), turn, "an ordinary answer", nothingWithheld); err != nil {
		t.Fatalf("sendReply: %v", err)
	}
	if turn.edited != "" {
		t.Errorf("edited %q with the flag off", turn.edited)
	}
	if turn.reply != "an ordinary answer" {
		t.Errorf("reply = %q, want the ordinary post", turn.reply)
	}
}

// A thread with no reply of ours yet has nothing to revise, and the first turn
// must post so there is something to edit next time.
func TestTheFirstTurnInAThreadStillPosts(t *testing.T) {
	t.Parallel()
	agent := summaryAgent(t, true)
	turn := summaryTurn("thread-1")

	if err := agent.sendReply(context.Background(), turn, "the first answer", nothingWithheld); err != nil {
		t.Fatalf("sendReply: %v", err)
	}
	if turn.edited != "" {
		t.Errorf("edited %q when the thread had no reply of ours", turn.edited)
	}
	if turn.reply != "the first answer" {
		t.Errorf("reply = %q, want the first answer posted", turn.reply)
	}
}

// Outside a thread there is no living answer, so a channel turn is untouched.
func TestAChannelTurnIsNeverRevised(t *testing.T) {
	t.Parallel()
	agent := summaryAgent(t, true)
	agent.summaries.Remember("", "reply-1")
	turn := summaryTurn("")

	if err := agent.sendReply(context.Background(), turn, "a channel answer", nothingWithheld); err != nil {
		t.Fatalf("sendReply: %v", err)
	}
	if turn.edited != "" {
		t.Errorf("edited %q outside a thread", turn.edited)
	}
}

// A deleted message must not cost the turn its answer, and must not be retried
// on every later turn in that thread.
func TestAFailedEditFallsBackAndForgets(t *testing.T) {
	t.Parallel()
	agent := summaryAgent(t, true)
	agent.summaries.Remember("thread-1", "reply-1")
	turn := summaryTurn("thread-1")
	turn.failEdit = true

	if err := agent.sendReply(context.Background(), turn, "the answer", nothingWithheld); err != nil {
		t.Fatalf("sendReply: %v", err)
	}
	if turn.reply != "the answer" {
		t.Errorf("reply = %q, want the answer posted after the edit failed", turn.reply)
	}
	if _, held := agent.summaries.First("thread-1"); held {
		t.Error("the unusable message is still remembered, so every later turn retries it")
	}
}

// The message a turn edits is the first this service posted, never the newest,
// because the living answer is one message rather than a moving target.
func TestOnlyTheFirstReplyIsRemembered(t *testing.T) {
	t.Parallel()
	summaries := newThreadSummaries(8)
	summaries.Remember("thread-1", "reply-1")
	summaries.Remember("thread-1", "reply-2")

	if got, _ := summaries.First("thread-1"); got != "reply-1" {
		t.Errorf("first = %q, want the earliest reply", got)
	}
}

// The cache is bounded like every other per-channel one, so a busy guild
// cannot grow process memory without limit.
func TestTheSummaryCacheIsBounded(t *testing.T) {
	t.Parallel()
	summaries := newThreadSummaries(4)
	for index := range 10 {
		summaries.Remember(string(rune('a'+index)), "reply")
	}
	if len(summaries.first) > 4 {
		t.Errorf("cache holds %d threads, want at most 4", len(summaries.first))
	}
}

// The edit goes through sendReply, so a blank answer is still stopped there
// rather than silently rewriting the living answer to nothing.
func TestABlankAnswerDoesNotWipeTheLivingAnswer(t *testing.T) {
	t.Parallel()
	agent := summaryAgent(t, true)
	agent.summaries.Remember("thread-1", "reply-1")
	turn := summaryTurn("thread-1")

	_ = agent.sendReply(context.Background(), turn, "   ", nothingWithheld)

	if strings.TrimSpace(turn.body) == "" && turn.edited != "" {
		t.Error("a blank answer reached the edit, so the living answer can be wiped")
	}
}
