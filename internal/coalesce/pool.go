package coalesce

import (
	"context"
	"log/slog"
	"sync"
	"time"
)

// Runner answers one batch in one agent turn. The batch carries the attempt,
// so the tier and the thinking setting reach the model without a second call.
type Runner interface {
	Run(ctx context.Context, batch Batch) error
}

// DeadLetter takes a batch the ladder gave up on. The member is told it is
// still queued, because silence reads as being ignored.
type DeadLetter interface {
	Shelve(ctx context.Context, batch Batch, cause error)
}

// discardLetters is the default for a deployment with no dead-letter surface.
type discardLetters struct{}

func (discardLetters) Shelve(context.Context, Batch, error) {}

// Pool drains batches across a fixed set of workers, one turn per batch, with
// at most one writer per tenant at a time.
type Pool struct {
	policy   Policy
	runner   Runner
	locks    *tenantLocks
	dead     DeadLetter
	log      Logger
	observer Observer
	clock    func() time.Time
}

// NewPool builds the pool. A nil dead-letter sink, logger, or observer records
// nothing rather than panicking on a path that already owes an answer.
func NewPool(policy Policy, runner Runner, dead DeadLetter, log Logger, observer Observer) *Pool {
	if dead == nil {
		dead = discardLetters{}
	}
	if log == nil {
		log = quiet{}
	}
	if observer == nil {
		observer = discard{}
	}
	return &Pool{
		policy:   policy.withDefaults(),
		runner:   runner,
		locks:    newTenantLocks(),
		dead:     dead,
		log:      log,
		observer: observer,
	}
}

// Start launches the workers and returns a function that blocks until they
// finish, which is after the batch channel closes and its last turn ends.
func (p *Pool) Start(ctx context.Context, batches <-chan Batch) func() {
	var wg sync.WaitGroup
	for i := 0; i < p.policy.Workers; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for batch := range batches {
				p.serve(ctx, batch)
			}
		}()
	}
	return wg.Wait
}

// serve runs one batch to a verdict. The tenant lock is held across the whole
// ladder, so a retry cannot interleave with the next batch for that member.
func (p *Pool) serve(ctx context.Context, batch Batch) {
	unlock := p.locks.Lock(batch.Tenant.Key())
	defer unlock()
	p.observer.Batch(ctx, batch.Size())
	var cause error
	for attempt := 1; attempt <= MaxAttempts; attempt++ {
		if ctx.Err() != nil {
			// A shutdown is not a failing batch, so it must not spend the
			// ladder that exists for one.
			cause = ctx.Err()
			break
		}
		batch.Attempt = attemptFor(attempt)
		if batch.Attempt.Tier == TierPro {
			p.observer.Escalated(ctx)
		}
		cause = p.attempt(ctx, batch)
		if cause == nil {
			return
		}
		p.log.Info(
			ctx,
			"coalesce.turn.retrying",
			slog.Int("attempt", batch.Attempt.Number),
			slog.String("tier", string(batch.Attempt.Tier)),
			slog.String("surface", batch.Tenant.Surface),
			slog.String("error", cause.Error()),
		)
	}
	p.shelve(ctx, batch, cause)
}

// attempt runs one turn under the hard deadline and records what it cost.
func (p *Pool) attempt(ctx context.Context, batch Batch) error {
	started := p.now()
	turnCtx, cancel := context.WithTimeout(ctx, p.policy.Deadline)
	defer cancel()
	err := p.runner.Run(turnCtx, batch)
	outcome := OutcomeServed
	if err != nil {
		outcome = OutcomeFailed
	}
	p.observer.Turn(ctx, outcome, batch.Attempt.Tier, p.now().Sub(started))
	return err
}

// shelve reports a batch the ladder exhausted and moves on, so one poisoned
// batch costs its own asks an answer rather than costing the pool a worker.
func (p *Pool) shelve(ctx context.Context, batch Batch, cause error) {
	p.observer.DeadLettered(ctx)
	p.observer.Turn(ctx, OutcomeAbandoned, batch.Attempt.Tier, 0)
	attrs := []slog.Attr{
		slog.String("error_type", "batch_abandoned"),
		slog.String("surface", batch.Tenant.Surface),
		slog.Int("asks", batch.Size()),
		slog.Int("attempts", batch.Attempt.Number),
	}
	if cause != nil {
		attrs = append(attrs, slog.String("error", cause.Error()))
	}
	p.log.Error(ctx, "coalesce.batch.abandoned", attrs...)
	// Detached from the turn context, because a batch abandoned on a deadline
	// would otherwise fail to say so for the same reason it failed.
	p.dead.Shelve(context.WithoutCancel(ctx), batch, cause)
}

func (p *Pool) now() time.Time {
	if p.clock != nil {
		return p.clock()
	}
	return time.Now().UTC()
}
