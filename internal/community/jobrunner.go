package community

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"sync"
	"time"
)

// The runner turns a submission into work that outlives its turn. See
// docs/sirens-echo-jobs-lifecycle.md.

const (
	// cancelPollInterval is how often a running job learns it was cancelled.
	cancelPollInterval = time.Second
)

// ErrJobQueueFull is returned when the runner will accept no more work.
var ErrJobQueueFull = errors.New("job queue is full")

// ErrNotJobOwner is returned when a principal acts on someone else's job.
var ErrNotJobOwner = errors.New("job belongs to another principal")

// JobExecutor performs one kind of work. Progress is advisory and may be
// dropped, so an executor must never depend on it being delivered.
type JobExecutor interface {
	Execute(ctx context.Context, job Job, progress func(string)) (string, error)
}

// JobNotifier delivers a terminal outcome to the origin a job came from.
type JobNotifier interface {
	NotifyJob(ctx context.Context, job Job) error
}

// JobRunner owns the queue, the workers, and the state transitions. It is the
// only thing that moves a job through its machine.
type JobRunner struct {
	Store     JobStore
	Telemetry *Telemetry
	Executors map[string]JobExecutor
	Notifier  JobNotifier
	// Grants decides what a principal may cause. Nil grants everything, which
	// is only correct before a deployment adopts a table.
	Grants *GrantTable
	// Progress delivers intermediate updates. Optional.
	Progress JobProgressReporter
	// Content delivers ordered answer messages. Optional, and inert without
	// ValidateContent, which fails closed rather than sending unchecked.
	Content JobContentReporter
	// ValidateContent runs the reply-path checks over one content message.
	ValidateContent func(string) error
	// Timeout bounds one execution. Zero uses defaultJobTimeout.
	Timeout time.Duration
	// Workers is the concurrent execution count. Zero means one.
	Workers int

	mu      sync.Mutex
	queue   chan string
	started bool
	stop    context.CancelFunc
	wait    sync.WaitGroup
	// cancels lets Cancel interrupt a running execution rather than waiting
	// for it to poll.
	cancels  map[string]context.CancelFunc
	progress *progressLimiter
	content  *contentCounter
}

func (r *JobRunner) timeout() time.Duration {
	if r.Timeout > 0 {
		return r.Timeout
	}
	return defaultJobTimeout
}

// Start brings up the workers on an empty queue. It requeues nothing, so a
// restart drops whatever was queued. See docs/sirens-echo-jobs.md.
func (r *JobRunner) Start(ctx context.Context) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.started {
		return nil
	}
	if r.Store == nil {
		return fmt.Errorf("job runner needs a store")
	}
	r.Telemetry = telemetryOrNoop(r.Telemetry)
	r.queue = make(chan string, defaultJobQueueDepth)
	r.cancels = make(map[string]context.CancelFunc)
	r.progress = newProgressLimiter(nil)
	r.content = newContentCounter(nil)
	runCtx, stop := context.WithCancel(context.WithoutCancel(ctx))
	r.stop = stop
	workers := r.Workers
	if workers < 1 {
		workers = 1
	}
	for worker := 0; worker < workers; worker++ {
		r.wait.Add(1)
		go r.work(runCtx)
	}
	r.started = true
	return nil
}

// Stop drains the workers. In-flight jobs are cancelled, and their records
// reach a terminal state rather than being left live.
func (r *JobRunner) Stop() {
	r.mu.Lock()
	stop, started := r.stop, r.started
	r.started = false
	r.mu.Unlock()
	if !started {
		return
	}
	stop()
	r.wait.Wait()
}

// Submit records the job and queues it. A redelivery returns the existing job
// and queues nothing, which is what makes at-least-once transports safe.
func (r *JobRunner) Submit(ctx context.Context, submission Submission) (Job, error) {
	prepared, err := PrepareJob(submission)
	if err != nil {
		return Job{}, err
	}
	if _, known := r.Executors[prepared.Kind]; !known {
		return Job{}, fmt.Errorf("no executor for job kind %q", prepared.Kind)
	}
	// A denial is a recorded outcome with a reason, so a refused request is
	// attributable rather than invisible. See docs/sirens-echo-grants.md.
	if denial := r.permits(prepared); denial != nil {
		refused, _, storeErr := r.Store.Submit(prepared)
		if storeErr != nil {
			return Job{}, denial
		}
		_, _ = r.Store.Transition(refused.ID, JobFailed, func(target *Job) {
			target.Outcome = "not permitted"
		})
		r.Telemetry.RecordJob(ctx, prepared.Kind, string(JobFailed))
		r.Telemetry.Info(ctx, "job.denied",
			slog.String("job_id", refused.ID), slog.String("kind", prepared.Kind))
		return Job{}, denial
	}
	job, existed, err := r.Store.Submit(prepared)
	if err != nil {
		return Job{}, err
	}
	if existed {
		r.Telemetry.Info(ctx, "job.submit.deduplicated",
			slog.String("job_id", job.ID), slog.String("state", string(job.State)))
		return job, nil
	}
	if err := r.enqueue(job.ID); err != nil {
		// A job that cannot be queued must not sit queued forever.
		_, _ = r.Store.Transition(job.ID, JobFailed, func(target *Job) {
			target.Outcome = "queue full"
		})
		return Job{}, err
	}
	r.Telemetry.RecordJob(ctx, job.Kind, string(JobQueued))
	r.Telemetry.Info(ctx, "job.submitted",
		slog.String("job_id", job.ID),
		slog.String("kind", job.Kind),
		slog.String("transport", job.Origin.Transport))
	return job, nil
}

// permits applies the grant table. No table grants everything, which is only
// correct before a deployment adopts one.
func (r *JobRunner) permits(job Job) error {
	if r.Grants == nil {
		return nil
	}
	if err := r.Grants.Permits(job.Principal, job.Kind); err != nil {
		return err
	}
	return nil
}

func (r *JobRunner) enqueue(id string) error {
	r.mu.Lock()
	queue := r.queue
	r.mu.Unlock()
	if queue == nil {
		return fmt.Errorf("job runner is not started")
	}
	select {
	case queue <- id:
		return nil
	default:
		return ErrJobQueueFull
	}
}

// Get returns a job the principal is allowed to see.
func (r *JobRunner) Get(id, principal string) (Job, error) {
	job, err := r.Store.Get(id)
	if err != nil {
		return Job{}, err
	}
	if job.Principal != principal {
		return Job{}, ErrNotJobOwner
	}
	return job, nil
}

// Cancel asks a job to stop. Queued work stops immediately because nothing
// started; running work is asked and stops cooperatively.
func (r *JobRunner) Cancel(ctx context.Context, id, principal string) (Job, error) {
	job, err := r.Get(id, principal)
	if err != nil {
		return Job{}, err
	}
	if job.State.Terminal() {
		return job, nil
	}
	next := JobCancelling
	if job.State == JobQueued {
		next = JobCancelled
	}
	moved, err := r.Store.Transition(id, next, func(target *Job) {
		target.Outcome = "cancelled by the requester"
	})
	if err != nil {
		return Job{}, err
	}
	r.mu.Lock()
	interrupt := r.cancels[id]
	r.mu.Unlock()
	if interrupt != nil {
		interrupt()
	}
	r.Telemetry.RecordJob(ctx, moved.Kind, string(moved.State))
	r.Telemetry.Info(ctx, "job.cancel.requested",
		slog.String("job_id", id), slog.String("state", string(moved.State)))
	if moved.State.Terminal() {
		r.notify(ctx, moved)
	}
	return moved, nil
}

func (r *JobRunner) work(ctx context.Context) {
	defer r.wait.Done()
	r.mu.Lock()
	queue := r.queue
	r.mu.Unlock()
	for {
		select {
		case <-ctx.Done():
			return
		case id := <-queue:
			r.run(ctx, id)
		}
	}
}

// run executes one job and lands it in a terminal state. Every exit path is a
// transition, so a job cannot end live.
func (r *JobRunner) run(parent context.Context, id string) {
	job, err := r.Store.Get(id)
	if err != nil {
		r.Telemetry.Error(parent, "job.missing", slog.String("job_id", id))
		return
	}
	// Cancelled while queued, so there is nothing to run.
	if job.State.Terminal() {
		return
	}
	running, err := r.Store.Transition(id, JobRunning, nil)
	if err != nil {
		if !IsJobTransitionError(err) {
			r.Telemetry.Error(parent, "job.start.failed", slog.String("job_id", id))
		}
		return
	}
	r.Telemetry.RecordJob(parent, running.Kind, string(JobRunning))

	jobCtx, cancel := context.WithTimeout(parent, r.timeout())
	r.mu.Lock()
	r.cancels[id] = cancel
	r.mu.Unlock()
	defer func() {
		cancel()
		r.mu.Lock()
		delete(r.cancels, id)
		limiter := r.progress
		counter := r.content
		r.mu.Unlock()
		if limiter != nil {
			limiter.forget(id)
		}
		if counter != nil {
			counter.forget(id)
		}
	}()

	jobCtx, span := r.Telemetry.StartJobSpan(jobCtx, running)
	watch := r.watchCancellation(jobCtx, id, cancel)
	outcome, execErr := r.execute(jobCtx, running)
	close(watch)
	span.End()

	final, err := r.settle(parent, id, outcome, execErr)
	if err != nil {
		r.Telemetry.Error(parent, "job.settle.failed", slog.String("job_id", id))
		return
	}
	r.notify(parent, final)
}

// execute runs the kind's executor, containing a panic so one job cannot take
// the worker down with it.
func (r *JobRunner) execute(ctx context.Context, job Job) (outcome string, err error) {
	defer func() {
		if recovered := recover(); recovered != nil {
			err = fmt.Errorf("job executor panicked")
		}
	}()
	executor, known := r.Executors[job.Kind]
	if !known {
		return "", fmt.Errorf("no executor for job kind %q", job.Kind)
	}
	return executor.Execute(ctx, job, r.progressFor(ctx, job))
}

// settle maps an execution result onto the terminal state. A cancelled job
// reports cancellation even if the executor returned an error on the way out.
func (r *JobRunner) settle(ctx context.Context, id, outcome string, execErr error) (Job, error) {
	current, err := r.Store.Get(id)
	if err != nil {
		return Job{}, err
	}
	if current.State == JobCancelling {
		return r.Store.Transition(id, JobCancelled, func(target *Job) {
			target.Outcome = "cancelled by the requester"
		})
	}
	if execErr != nil {
		phrase := "job failed"
		if errors.Is(execErr, context.DeadlineExceeded) {
			phrase = "job timed out"
		}
		r.Telemetry.RecordJob(ctx, current.Kind, string(JobFailed))
		return r.Store.Transition(id, JobFailed, func(target *Job) {
			target.Outcome = phrase
		})
	}
	r.Telemetry.RecordJob(ctx, current.Kind, string(JobSucceeded))
	return r.Store.Transition(id, JobSucceeded, func(target *Job) {
		target.Outcome = outcome
	})
}

// watchCancellation interrupts a running execution when the record moves to
// cancelling, so a cancel does not wait out the job's own timeout.
func (r *JobRunner) watchCancellation(
	ctx context.Context,
	id string,
	cancel context.CancelFunc,
) chan struct{} {
	done := make(chan struct{})
	go func() {
		ticker := time.NewTicker(cancelPollInterval)
		defer ticker.Stop()
		for {
			select {
			case <-done:
				return
			case <-ctx.Done():
				return
			case <-ticker.C:
				job, err := r.Store.Get(id)
				if err == nil && job.State == JobCancelling {
					cancel()
					return
				}
			}
		}
	}()
	return done
}

func (r *JobRunner) notify(ctx context.Context, job Job) {
	if r.Notifier == nil || !job.Notifiable() {
		return
	}
	// Detached from the worker, so a shutdown cannot swallow the one message
	// telling the requester how their job went.
	notifyCtx, cancel := context.WithTimeout(context.WithoutCancel(ctx), failureNoticeTimeout)
	defer cancel()
	if err := r.Notifier.NotifyJob(notifyCtx, job); err != nil {
		r.Telemetry.RecordFailure(notifyCtx, "job_notify")
		r.Telemetry.Error(notifyCtx, "job.notify.failed", slog.String("job_id", job.ID))
	}
}
