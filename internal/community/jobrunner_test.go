package community

import (
	"context"
	"errors"
	"sync"
	"testing"
	"time"
)

// blockingExecutor holds until released, so a test can observe a running job
// and cancel it deterministically instead of racing a sleep.
type blockingExecutor struct {
	started chan struct{}
	release chan struct{}
	once    sync.Once
}

func newBlockingExecutor() *blockingExecutor {
	return &blockingExecutor{
		started: make(chan struct{}),
		release: make(chan struct{}),
	}
}

func (e *blockingExecutor) Execute(
	ctx context.Context,
	_ Job,
	progress func(string),
) (string, error) {
	e.once.Do(func() { close(e.started) })
	if progress != nil {
		progress("working")
	}
	select {
	case <-ctx.Done():
		return "", ctx.Err()
	case <-e.release:
		return "done", nil
	}
}

type recordingNotifier struct {
	mu   sync.Mutex
	jobs []Job
	done chan struct{}
	once sync.Once
}

func newRecordingNotifier() *recordingNotifier {
	return &recordingNotifier{done: make(chan struct{})}
}

func (n *recordingNotifier) NotifyJob(_ context.Context, job Job) error {
	n.mu.Lock()
	n.jobs = append(n.jobs, job)
	n.mu.Unlock()
	n.once.Do(func() { close(n.done) })
	return nil
}

func (n *recordingNotifier) waitForOne(t *testing.T) Job {
	t.Helper()
	select {
	case <-n.done:
	case <-time.After(5 * time.Second):
		t.Fatal("no completion notice reached the origin")
	}
	n.mu.Lock()
	defer n.mu.Unlock()
	return n.jobs[0]
}

func startedRunner(t *testing.T, executor JobExecutor, notifier JobNotifier) *JobRunner {
	t.Helper()
	runner := &JobRunner{
		Store:     NewMemoryJobStore(nil),
		Executors: map[string]JobExecutor{"echo": executor},
		Notifier:  notifier,
		Timeout:   5 * time.Second,
	}
	if err := runner.Start(context.Background()); err != nil {
		t.Fatalf("Start: %v", err)
	}
	t.Cleanup(runner.Stop)
	return runner
}

func waitForState(t *testing.T, runner *JobRunner, id, principal string, want JobState) Job {
	t.Helper()
	deadline := time.Now().Add(5 * time.Second)
	for time.Now().Before(deadline) {
		job, err := runner.Get(id, principal)
		if err != nil {
			t.Fatalf("Get: %v", err)
		}
		if job.State == want {
			return job
		}
		time.Sleep(5 * time.Millisecond)
	}
	job, _ := runner.Get(id, principal)
	t.Fatalf("job never reached %s, last state %s", want, job.State)
	return Job{}
}

// Submitting long work returns an id without holding the transport open.
func TestSubmitReturnsImmediatelyAndCompletionNotifiesTheOrigin(t *testing.T) {
	t.Parallel()
	executor := newBlockingExecutor()
	notifier := newRecordingNotifier()
	runner := startedRunner(t, executor, notifier)

	job, err := runner.Submit(context.Background(), discordSubmission())
	if err != nil {
		t.Fatalf("Submit: %v", err)
	}
	if job.ID == "" || job.State != JobQueued {
		t.Fatalf("submitted job = %#v", job)
	}

	select {
	case <-executor.started:
	case <-time.After(5 * time.Second):
		t.Fatal("the job never started")
	}
	waitForState(t, runner, job.ID, job.Principal, JobRunning)
	close(executor.release)

	final := waitForState(t, runner, job.ID, job.Principal, JobSucceeded)
	if final.Outcome != "done" {
		t.Errorf("outcome = %q", final.Outcome)
	}
	notified := notifier.waitForOne(t)
	if notified.ID != job.ID || notified.State != JobSucceeded {
		t.Errorf("notified job = %#v", notified)
	}
}

// A cancelled job stops, reaches a terminal state, and does not later report
// success, which is the acceptance #144 is most specific about.
func TestCancellingARunningJobReachesCancelledAndNotSucceeded(t *testing.T) {
	t.Parallel()
	executor := newBlockingExecutor()
	notifier := newRecordingNotifier()
	runner := startedRunner(t, executor, notifier)

	job, err := runner.Submit(context.Background(), discordSubmission())
	if err != nil {
		t.Fatalf("Submit: %v", err)
	}
	select {
	case <-executor.started:
	case <-time.After(5 * time.Second):
		t.Fatal("the job never started")
	}
	waitForState(t, runner, job.ID, job.Principal, JobRunning)

	if _, err := runner.Cancel(context.Background(), job.ID, job.Principal); err != nil {
		t.Fatalf("Cancel: %v", err)
	}
	final := waitForState(t, runner, job.ID, job.Principal, JobCancelled)
	if final.Outcome == "" {
		t.Error("a cancelled job did not say why")
	}
	notified := notifier.waitForOne(t)
	if notified.State == JobSucceeded {
		t.Error("a cancelled job reported success")
	}
	// Releasing afterwards must not resurrect it.
	close(executor.release)
	time.Sleep(50 * time.Millisecond)
	after, err := runner.Get(job.ID, job.Principal)
	if err != nil {
		t.Fatalf("Get: %v", err)
	}
	if after.State != JobCancelled {
		t.Errorf("state after release = %s", after.State)
	}
}

// Cancelling queued work needs no cooperative stop, because nothing started.
func TestCancellingAQueuedJobIsImmediate(t *testing.T) {
	t.Parallel()
	blocker := newBlockingExecutor()
	notifier := newRecordingNotifier()
	runner := startedRunner(t, blocker, notifier)

	// Occupy the single worker so the second job stays queued.
	first, err := runner.Submit(context.Background(), discordSubmission())
	if err != nil {
		t.Fatalf("Submit: %v", err)
	}
	select {
	case <-blocker.started:
	case <-time.After(5 * time.Second):
		t.Fatal("the first job never started")
	}

	queuedSubmission := discordSubmission()
	queuedSubmission.Origin.MessageID = "1537024279743434900"
	second, err := runner.Submit(context.Background(), queuedSubmission)
	if err != nil {
		t.Fatalf("Submit: %v", err)
	}
	cancelled, err := runner.Cancel(context.Background(), second.ID, second.Principal)
	if err != nil {
		t.Fatalf("Cancel: %v", err)
	}
	if cancelled.State != JobCancelled {
		t.Errorf("queued cancel landed in %s, want cancelled", cancelled.State)
	}
	if cancelled.Attempts != 0 {
		t.Errorf("a queued job that was cancelled recorded %d attempts", cancelled.Attempts)
	}
	close(blocker.release)
	waitForState(t, runner, first.ID, first.Principal, JobSucceeded)
}

// A job that outruns its bound fails, rather than running forever.
func TestAJobThatOverrunsItsTimeoutFails(t *testing.T) {
	t.Parallel()
	executor := newBlockingExecutor()
	notifier := newRecordingNotifier()
	runner := &JobRunner{
		Store:     NewMemoryJobStore(nil),
		Executors: map[string]JobExecutor{"echo": executor},
		Notifier:  notifier,
		Timeout:   50 * time.Millisecond,
	}
	if err := runner.Start(context.Background()); err != nil {
		t.Fatalf("Start: %v", err)
	}
	t.Cleanup(runner.Stop)

	job, err := runner.Submit(context.Background(), discordSubmission())
	if err != nil {
		t.Fatalf("Submit: %v", err)
	}
	final := waitForState(t, runner, job.ID, job.Principal, JobFailed)
	if final.Outcome != "job timed out" {
		t.Errorf("outcome = %q, want the timeout phrase", final.Outcome)
	}
	close(executor.release)
}

// One principal must not read or cancel another's job.
func TestJobsAreScopedToTheirRequester(t *testing.T) {
	t.Parallel()
	runner := startedRunner(t, EchoJobExecutor{}, newRecordingNotifier())
	job, err := runner.Submit(context.Background(), discordSubmission())
	if err != nil {
		t.Fatalf("Submit: %v", err)
	}
	if _, err := runner.Get(job.ID, "someone-else"); !errors.Is(err, ErrNotJobOwner) {
		t.Errorf("Get by another principal = %v", err)
	}
	if _, err := runner.Cancel(context.Background(), job.ID, "someone-else"); !errors.Is(err, ErrNotJobOwner) {
		t.Errorf("Cancel by another principal = %v", err)
	}
}

// A redelivered submission must not start a second execution.
func TestRedeliveryDoesNotQueueASecondExecution(t *testing.T) {
	t.Parallel()
	counter := &countingExecutor{}
	runner := startedRunner(t, counter, newRecordingNotifier())

	first, err := runner.Submit(context.Background(), discordSubmission())
	if err != nil {
		t.Fatalf("Submit: %v", err)
	}
	waitForState(t, runner, first.ID, first.Principal, JobSucceeded)
	second, err := runner.Submit(context.Background(), discordSubmission())
	if err != nil {
		t.Fatalf("resubmit: %v", err)
	}
	if second.ID != first.ID {
		t.Fatalf("redelivery made a second job: %s", second.ID)
	}
	time.Sleep(50 * time.Millisecond)
	if runs := counter.count(); runs != 1 {
		t.Errorf("executor ran %d times for one request", runs)
	}
}

type countingExecutor struct {
	mu   sync.Mutex
	runs int
}

func (e *countingExecutor) Execute(context.Context, Job, func(string)) (string, error) {
	e.mu.Lock()
	defer e.mu.Unlock()
	e.runs++
	return "counted", nil
}

func (e *countingExecutor) count() int {
	e.mu.Lock()
	defer e.mu.Unlock()
	return e.runs
}

// An executor that panics must not take the worker with it.
func TestAPanickingExecutorFailsOnlyItsOwnJob(t *testing.T) {
	t.Parallel()
	runner := startedRunner(t, panickingExecutor{}, newRecordingNotifier())
	job, err := runner.Submit(context.Background(), discordSubmission())
	if err != nil {
		t.Fatalf("Submit: %v", err)
	}
	final := waitForState(t, runner, job.ID, job.Principal, JobFailed)
	if final.Outcome != "job failed" {
		t.Errorf("outcome = %q", final.Outcome)
	}
}

type panickingExecutor struct{}

func (panickingExecutor) Execute(context.Context, Job, func(string)) (string, error) {
	panic("executor exploded")
}

// Progress is rate limited, so a chatty job cannot flood its origin.
func TestProgressIsRateLimitedPerJob(t *testing.T) {
	t.Parallel()
	limiter := newProgressLimiter(nil)
	if !limiter.admit("job-aaaaaaaa") {
		t.Fatal("the first update was dropped")
	}
	if limiter.admit("job-aaaaaaaa") {
		t.Error("a second update inside the window was admitted")
	}
	if !limiter.admit("job-bbbbbbbb") {
		t.Error("another job was throttled by the first job's update")
	}
	limiter.forget("job-aaaaaaaa")
	if !limiter.admit("job-aaaaaaaa") {
		t.Error("a forgotten job stayed throttled")
	}
}

// The job id has to reach logs and spans, which is how a run is retrievable.
func TestJobIDRidesTheContext(t *testing.T) {
	t.Parallel()
	ctx := ContextWithJobID(context.Background(), "job-0123456789")
	if got := JobIDFromContext(ctx); got != "job-0123456789" {
		t.Errorf("job id from context = %q", got)
	}
	if got := JobIDFromContext(context.Background()); got != "" {
		t.Errorf("job id outside a job = %q", got)
	}
}

// The runner's own comment claimed Start requeued these, and it never did. See
// sirens-echo#824 and docs/sirens-echo-jobs.md.
func TestARestartDropsWhatWasQueued(t *testing.T) {
	t.Parallel()
	store := NewMemoryJobStore(nil)
	queued := submitTestJob(t, store)
	if queued.State != JobQueued {
		t.Fatalf("fixture is not queued: %s", queued.State)
	}

	executor := newBlockingExecutor()
	runner := &JobRunner{
		Store:     store,
		Executors: map[string]JobExecutor{"echo": executor},
		Timeout:   5 * time.Second,
	}
	if err := runner.Start(context.Background()); err != nil {
		t.Fatalf("Start: %v", err)
	}
	t.Cleanup(runner.Stop)

	// Long enough for a worker to have picked it up if anything had queued it.
	select {
	case <-executor.started:
		t.Fatal("Start ran a job the store already held, so this behaviour changed")
	case <-time.After(200 * time.Millisecond):
	}
	current, err := store.Get(queued.ID)
	if err != nil {
		t.Fatalf("Get: %v", err)
	}
	if current.State != JobQueued {
		t.Errorf("a restart moved the queued job to %s", current.State)
	}
}
