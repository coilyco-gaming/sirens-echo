package ingest

import (
	"context"
	"log/slog"
	"sync/atomic"
)

// Ingress admits one ask at a time and is the only place an ack is applied.
// The order inside Submit is the contract rather than a convenience.
type Ingress struct {
	queue    *Queue
	ack      Acknowledger
	log      Logger
	observer Observer
	clock    Clock
	seq      atomic.Int64
}

// NewIngress wires an ingress onto a queue. A nil acknowledger is the silent
// one, and a nil logger or observer records nothing.
func NewIngress(queue *Queue, ack Acknowledger, log Logger, observer Observer, clock Clock) *Ingress {
	if queue == nil {
		queue = NewQueue(DefaultCapacity)
	}
	if ack == nil {
		ack = Silent{}
	}
	if log == nil {
		log = quiet{}
	}
	if observer == nil {
		observer = discard{}
	}
	return &Ingress{queue: queue, ack: ack, log: log, observer: observer, clock: clock}
}

// Submit stamps, acknowledges, and queues one ask. It never blocks on the
// coalescer, so a gateway handler calling it cannot stall behind a slow turn.
func (i *Ingress) Submit(ctx context.Context, tenant Tenant, locus, text string, subject Subject) Ask {
	ask := Ask{
		Seq:     i.seq.Add(1),
		Tenant:  tenant,
		Locus:   locus,
		Text:    text,
		At:      i.clock.now(),
		Subject: subject,
	}
	// Before the queue, not after. An ack applied on the far side would be an
	// ack for whatever survived the bound rather than for the comment.
	if err := i.ack.Queued(ctx, ask); err != nil {
		i.log.Info(
			ctx,
			"ingest.ack.failed",
			slog.String("surface", tenant.Surface),
			slog.String("error", err.Error()),
		)
	}
	shed, dropped := i.queue.Push(ask)
	if dropped {
		i.shed(ctx, shed)
	}
	i.observer.Accepted(ctx, tenant.Surface)
	i.observer.Depth(ctx, i.queue.Depth())
	return ask
}

// shed reports a dropped ask and retracts its mark, which was a promise of an
// answer no worker will now produce.
func (i *Ingress) shed(ctx context.Context, ask Ask) {
	i.observer.Shed(ctx, ask.Tenant.Surface)
	i.log.Error(
		ctx,
		"ingest.queue.shed",
		slog.String("error_type", "queue_overflow"),
		slog.String("surface", ask.Tenant.Surface),
		slog.Int64("seq", ask.Seq),
		slog.Int("capacity", i.queue.Capacity()),
	)
	if err := i.ack.Shed(ctx, ask); err != nil {
		i.log.Info(ctx, "ingest.shed.notice.failed", slog.String("error", err.Error()))
	}
}

// Queue exposes the buffer the coalescer drains and the depth widening reads.
func (i *Ingress) Queue() *Queue { return i.queue }
