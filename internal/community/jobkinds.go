package community

import (
	"context"
	"time"
)

// EchoJobExecutor proves the lifecycle end to end without touching anything.
// It exists so the machine can be exercised before any real work runs.
type EchoJobExecutor struct {
	// Delay makes the job long enough to observe, cancel, and time out.
	Delay time.Duration
}

// Execute waits out its delay, reporting progress and honouring cancellation.
func (e EchoJobExecutor) Execute(
	ctx context.Context,
	job Job,
	progress func(string),
) (string, error) {
	if progress != nil {
		progress("job started")
	}
	if e.Delay <= 0 {
		return "echoed", nil
	}
	timer := time.NewTimer(e.Delay)
	defer timer.Stop()
	select {
	case <-ctx.Done():
		return "", ctx.Err()
	case <-timer.C:
		if progress != nil {
			progress("job finishing")
		}
		return "echoed", nil
	}
}

// DefaultJobExecutors is the executor set a deployment gets. It mirrors
// JobKinds, and a kind without an executor is refused at submission.
func DefaultJobExecutors() map[string]JobExecutor {
	return map[string]JobExecutor{
		"echo": EchoJobExecutor{},
	}
}
