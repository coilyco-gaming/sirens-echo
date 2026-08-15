package community

import (
	"context"
	"fmt"
	"log/slog"
	"strings"
	"sync"
	"time"
)

// Content is an answer, so it is ordered, never edited, and validated the way
// a reply is. Progress is a status line and stays what it was. See #356.

// JobContentReporter delivers one ordered message of a job's answer. It is a
// sibling of JobProgressReporter rather than a change to it.
type JobContentReporter interface {
	EmitJobContent(ctx context.Context, job Job, content string) error
}

// ErrJobContentExhausted ends a job that has said its ten messages, so the
// bound is a refusal the executor sees rather than a silent drop.
var ErrJobContentExhausted = fmt.Errorf("job reached its %d message limit", maxJobContentMessages)

// ErrJobContentWindowClosed is the time half, separate so an executor and an
// operator can tell which ceiling ended the answer.
var ErrJobContentWindowClosed = fmt.Errorf("job reached its %s answer window", maxJobContentWindow)

// contentCounter bounds one job's answer by count and by elapsed time. The
// progress limiter is a rate and drops; this is a total and refuses.
type contentCounter struct {
	mu      sync.Mutex
	sent    map[string]int
	started map[string]time.Time
	now     Clock
}

func newContentCounter(now Clock) *contentCounter {
	return &contentCounter{
		sent:    make(map[string]int),
		started: make(map[string]time.Time),
		now:     now,
	}
}

func (c *contentCounter) moment() time.Time {
	if c.now != nil {
		return c.now()
	}
	return time.Now().UTC()
}

// contentEffectStep names the message a job has already delivered. Keyed by
// position, because that is what a re-execution repeats. See sirens-echo#621.
func contentEffectStep(sequence int) string {
	return fmt.Sprintf("content:%d", sequence)
}

// sequence reports how many messages this job has emitted, without admitting
// another. Used to name the effect before the send.
func (c *contentCounter) sequence(id string) int {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.sent[id]
}

// admit reports why this job may not emit, or nil. The window opens on the
// first message rather than at submission, so queue time is not answer time.
func (c *contentCounter) admit(id string) error {
	c.mu.Lock()
	defer c.mu.Unlock()
	moment := c.moment()
	opened, running := c.started[id]
	if !running {
		c.started[id] = moment
		c.sent[id] = 1
		return nil
	}
	if moment.Sub(opened) >= maxJobContentWindow {
		return ErrJobContentWindowClosed
	}
	if c.sent[id] >= maxJobContentMessages {
		return ErrJobContentExhausted
	}
	c.sent[id]++
	return nil
}

func (c *contentCounter) forget(id string) {
	c.mu.Lock()
	defer c.mu.Unlock()
	delete(c.sent, id)
	delete(c.started, id)
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
		if counter == nil {
			return ErrJobContentExhausted
		}
		if err := counter.admit(job.ID); err != nil {
			r.Telemetry.Info(ctx, "job.content.exhausted",
				slog.String("job_id", job.ID), slog.String("ceiling", err.Error()))
			return err
		}
		// A re-execution repeats the executor's emits. Recorded per position, so
		// the second run skips what the first delivered. See sirens-echo#621.
		step := contentEffectStep(counter.sequence(job.ID))
		if r.Store != nil {
			current, err := r.Store.Get(job.ID)
			if err == nil && EffectApplied(current, step) {
				r.Telemetry.Info(ctx, "job.content.replayed",
					slog.String("job_id", job.ID), slog.String("step", step))
				return nil
			}
		}
		r.Telemetry.Info(ctx, "job.content.emitted",
			slog.String("job_id", job.ID), slog.Int("content_bytes", len(content)))
		// Not detached from the job. An answer a cancelled job is still writing
		// is one the member did not ask to finish, unlike a progress line.
		if err := r.Content.EmitJobContent(ctx, job, content); err != nil {
			r.Telemetry.RecordFailure(ctx, "job_content")
			return err
		}
		// After the send. Recording first would skip a message the origin never
		// received, which is the failure this guard exists to avoid.
		if r.Store != nil {
			if _, err := RecordEffect(r.Store, job.ID, step, "delivered"); err != nil {
				r.Telemetry.Info(ctx, "job.content.effect.failed",
					slog.String("job_id", job.ID), slog.String("step", step))
			}
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
