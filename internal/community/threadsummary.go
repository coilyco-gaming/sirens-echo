package community

import (
	"context"
	"sync"

	"github.com/bwmarrin/discordgo"
)

// A thread can carry one living answer instead of a run of replies. See
// docs/sirens-echo-threads.md and sirens-echo#951.

// threadSummaries remembers this service's first reply in each thread, so a
// later turn there can edit it. Bounded like every other per-channel cache.
type threadSummaries struct {
	mu       sync.Mutex
	capacity int
	order    []string
	first    map[string]string
}

func newThreadSummaries(capacity int) *threadSummaries {
	if capacity <= 0 {
		capacity = 256
	}
	return &threadSummaries{
		capacity: capacity,
		order:    make([]string, 0, capacity),
		first:    make(map[string]string, capacity),
	}
}

// Remember records a reply only when the thread has none, because the message
// a later turn edits is the first one rather than the newest.
func (s *threadSummaries) Remember(threadID, messageID string) {
	if threadID == "" || messageID == "" {
		return
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if _, held := s.first[threadID]; held {
		return
	}
	s.first[threadID] = messageID
	s.order = append(s.order, threadID)
	for len(s.order) > s.capacity {
		oldest := s.order[0]
		s.order = s.order[1:]
		delete(s.first, oldest)
	}
}

// First returns the message a turn in this thread would edit, if any.
func (s *threadSummaries) First(threadID string) (string, bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	messageID, held := s.first[threadID]
	return messageID, held
}

// Forget drops a thread's record, so a message this service can no longer edit
// is not retried on every turn.
func (s *threadSummaries) Forget(threadID string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	delete(s.first, threadID)
	for index, held := range s.order {
		if held == threadID {
			s.order = append(s.order[:index], s.order[index+1:]...)
			break
		}
	}
}

// replyEditor is an optional turn capability. A transport that can revise a
// message it already sent declares it.
type replyEditor interface {
	EditReply(ctx context.Context, messageID, content string) error
}

// EditReply revises a message this service posted. Discord refuses another
// author's message, which is why the thread's own starter is never the target.
func (t *discordMessageTurn) EditReply(_ context.Context, messageID, content string) error {
	body := truncateRunes(content, discordReplyLimit)
	_, err := t.session.ChannelMessageEditComplex(&discordgo.MessageEdit{
		Channel: t.message.ChannelID,
		ID:      messageID,
		Content: &body,
		AllowedMentions: &discordgo.MessageAllowedMentions{
			Parse: []discordgo.AllowedMentionType{},
		},
	})
	return err
}

// summaryTarget reports the message this turn should edit instead of posting.
// Empty means post, which is every turn outside an enabled thread.
func (a *Agent) summaryTarget(turn turnIO) string {
	if !a.cfg.ThreadSummary || a.summaries == nil {
		return ""
	}
	if _, ok := turn.(replyEditor); !ok {
		return ""
	}
	located, ok := turn.(threadScoped)
	if !ok {
		return ""
	}
	threadID := located.ThreadID()
	if threadID == "" {
		return ""
	}
	messageID, held := a.summaries.First(threadID)
	if !held {
		return ""
	}
	return messageID
}

// threadScoped is implemented by a turn that knows it is in a thread, which is
// the only place a living answer makes sense.
type threadScoped interface {
	ThreadID() string
}

// ThreadID names the thread this turn is in, or empty in a plain channel.
func (t *discordMessageTurn) ThreadID() string {
	if t.session == nil || t.session.State == nil || t.message == nil {
		return ""
	}
	channel, err := t.session.State.Channel(t.message.ChannelID)
	if err != nil || channel == nil || !channel.IsThread() {
		return ""
	}
	return channel.ID
}

// reviseSummary edits the thread's living answer, falling back to an ordinary
// reply when the edit fails, so a deleted message never costs the turn.
func (a *Agent) reviseSummary(
	ctx context.Context, turn turnIO, messageID, content string,
) error {
	editor, ok := turn.(replyEditor)
	if !ok {
		return turn.Reply(ctx, content)
	}
	if err := editor.EditReply(ctx, messageID, content); err == nil {
		return nil
	}
	// The message is gone or no longer ours, so this thread has no living
	// answer to revise and the next turn should not try again.
	if located, scoped := turn.(threadScoped); scoped {
		a.summaries.Forget(located.ThreadID())
	}
	a.telemetry.RecordFailure(ctx, "thread.summary.edit")
	return turn.Reply(ctx, content)
}
