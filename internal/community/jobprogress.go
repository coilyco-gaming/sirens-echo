package community

import (
	"context"
	"log/slog"
	"sync"
	"time"
)

// Progress is advisory. It is rate limited and droppable, so a chatty job
// cannot flood its origin. See docs/sirens-echo-jobs-lifecycle.md.

// JobProgressReporter delivers an intermediate update to a job's origin.
type JobProgressReporter interface {
	ReportJobProgress(ctx context.Context, job Job, phrase string) error
}

// progressLimiter drops updates that arrive inside the window. One per job,
// held by the runner for the life of the execution.
type progressLimiter struct {
	mu   sync.Mutex
	last map[string]time.Time
	now  Clock
}

func newProgressLimiter(now Clock) *progressLimiter {
	return &progressLimiter{last: make(map[string]time.Time), now: now}
}

func (l *progressLimiter) moment() time.Time {
	if l.now != nil {
		return l.now()
	}
	return time.Now().UTC()
}

// admit reports whether this job may report now, and records that it did.
func (l *progressLimiter) admit(id string) bool {
	l.mu.Lock()
	defer l.mu.Unlock()
	moment := l.moment()
	if previous, seen := l.last[id]; seen && moment.Sub(previous) < jobProgressEvery {
		return false
	}
	l.last[id] = moment
	return true
}

func (l *progressLimiter) forget(id string) {
	l.mu.Lock()
	defer l.mu.Unlock()
	delete(l.last, id)
}

// progressFor returns the callback handed to an executor. Every update is
// logged; only an admitted one reaches the origin.
func (r *JobRunner) progressFor(ctx context.Context, job Job) func(string) {
	return func(phrase string) {
		notice := harnessNotice(phrase)
		r.Telemetry.Info(ctx, "job.progress",
			slog.String("job_id", job.ID), slog.String("notice", notice))
		if r.Progress == nil || !job.Notifiable() {
			return
		}
		r.mu.Lock()
		limiter := r.progress
		r.mu.Unlock()
		if limiter == nil || !limiter.admit(job.ID) {
			return
		}
		// Detached from the job, so a cancellation does not swallow the update
		// explaining what the job was doing when it stopped.
		reportCtx, cancel := context.WithTimeout(
			context.WithoutCancel(ctx),
			failureNoticeTimeout,
		)
		defer cancel()
		if err := r.Progress.ReportJobProgress(reportCtx, job, notice); err != nil {
			r.Telemetry.RecordFailure(reportCtx, "job_progress")
		}
	}
}
