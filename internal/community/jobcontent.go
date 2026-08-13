package community

import (
	"context"
	"fmt"
	"log/slog"
	"strings"
	"sync"
)

// Content is an answer, so it is ordered, never edited, and validated the way
// a reply is. Progress is a status line and stays what it was. See #356.

// JobContentReporter delivers one ordered message of a job's answer. It is a
// sibling of JobProgressReporter rather than a change to it.
type JobContentReporter interface {
	EmitJobContent(ctx context.Context, job Job, content string) error
}

// maxJobContentMessages bounds one job's answer. The ceiling Kai decided on
// sirens-echo#236 is ten, and threading it is a separate change.
const maxJobContentMessages = 10

// ErrJobContentExhausted ends a job that has said its ten messages, so the
// bound is a refusal the executor sees rather than a silent drop.
var ErrJobContentExhausted = fmt.Errorf("job reached its %d message limit", maxJobContentMessages)

// contentCounter counts emitted messages per job. Separate from the progress
// limiter, which is a rate and drops; this is a total and refuses.
type contentCounter struct {
	mu   sync.Mutex
	sent map[string]int
}

func newContentCounter() *contentCounter {
	return &contentCounter{sent: make(map[string]int)}
}

// admit reports whether this job may emit again, and records that it did.
func (c *contentCounter) admit(id string) bool {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.sent[id] >= maxJobContentMessages {
		return false
	}
	c.sent[id]++
	return true
}

func (c *contentCounter) forget(id string) {
	c.mu.Lock()
	defer c.mu.Unlock()
	delete(c.sent, id)
}

// contentFor returns the emitter handed to an executor. It returns an error
// rather than dropping, because a missing paragraph is a hole in an answer.
func (r *JobRunner) contentFor(ctx context.Context, job Job) func(string) error {
	return func(content string) error {
		if strings.TrimSpace(content) == "" {
			return fmt.Errorf("job content is empty")
		}
		if r.Content == nil || !job.Notifiable() {
			return fmt.Errorf("job origin cannot receive content")
		}
		// Fail closed. An unwired validator must not become a path that reaches
		// a member unchecked, which is the shape sirens-echo#621 records.
		if r.ValidateContent == nil {
			return fmt.Errorf("job content validation is not configured")
		}
		if err := r.ValidateContent(content); err != nil {
			r.Telemetry.Info(ctx, "job.content.refused",
				slog.String("job_id", job.ID), slog.String("refused", err.Error()))
			return err
		}
		r.mu.Lock()
		counter := r.content
		r.mu.Unlock()
		if counter == nil || !counter.admit(job.ID) {
			r.Telemetry.Info(ctx, "job.content.exhausted",
				slog.String("job_id", job.ID))
			return ErrJobContentExhausted
		}
		r.Telemetry.Info(ctx, "job.content.emitted",
			slog.String("job_id", job.ID), slog.Int("content_bytes", len(content)))
		// Not detached from the job. An answer a cancelled job is still writing
		// is one the member did not ask to finish, unlike a progress line.
		if err := r.Content.EmitJobContent(ctx, job, content); err != nil {
			r.Telemetry.RecordFailure(ctx, "job_content")
			return err
		}
		return nil
	}
}

// EmitJobContent posts a new message every time. Editing would overwrite the
// paragraph before it, which is the whole difference from progress.
func (r *discordJobReporter) EmitJobContent(ctx context.Context, job Job, content string) error {
	if r.session == nil {
		return fmt.Errorf("no Discord session")
	}
	_, err := r.post(job, content)
	return err
}

// validateJobContent runs the turn-independent half of runReplyChecks. The
// order matches, so the two paths refuse in the same sequence.
func (a *Agent) validateJobContent(content string) error {
	reply, err := ParseReply(content)
	if err != nil {
		return err
	}
	// Grounding and self-attribution are absent by construction, not by
	// oversight: both read a turn's supplied context and executed tools.
	for _, check := range []func() error{
		func() error { return ValidateNoToolCallMarkup(reply) },
		func() error { return a.identifiers.Validate(reply) },
		func() error { return ValidateIdentityClaim(reply, a.cfg.Principal) },
		func() error { return ValidateResponseStyle(a.cfg.Definition.ResponseStyle, reply) },
	} {
		if err := check(); err != nil {
			return err
		}
	}
	return nil
}
