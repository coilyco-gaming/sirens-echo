package ingest

import (
	"context"
	"sync"
)

// DefaultCapacity bounds the asks waiting for the coalescer.
// TODO(prong-c): move to SIRENS_ECHO_COALESCE_CAPACITY, whose file Prong C owns.
const DefaultCapacity = 200

// Queue is a bounded ask buffer that sheds its oldest rather than blocking
// ingress. A channel cannot drop its head, which is why this is not one.
type Queue struct {
	mu     sync.Mutex
	items  []Ask
	head   int
	count  int
	closed bool
	// signal carries at most one wakeup. A consumer that finds the buffer empty
	// loops, so a coalesced wakeup can never strand an ask.
	signal chan struct{}
	// done wakes every waiter at once. One buffered signal wakes exactly one,
	// so a second consumer would wait for an ask that is never coming.
	done chan struct{}
}

// NewQueue builds a queue of the given capacity, or DefaultCapacity when the
// caller supplies nothing usable.
func NewQueue(capacity int) *Queue {
	if capacity <= 0 {
		capacity = DefaultCapacity
	}
	return &Queue{
		items:  make([]Ask, capacity),
		signal: make(chan struct{}, 1),
		done:   make(chan struct{}),
	}
}

// Push adds an ask, returning the ask it shed to make room. Ingress never
// blocks here, whatever the consumer is doing.
func (q *Queue) Push(ask Ask) (Ask, bool) {
	q.mu.Lock()
	shed, dropped := q.pushLocked(ask)
	q.mu.Unlock()
	q.wake()
	return shed, dropped
}

func (q *Queue) pushLocked(ask Ask) (Ask, bool) {
	var shed Ask
	dropped := false
	if q.count == len(q.items) {
		shed = q.items[q.head]
		q.head = (q.head + 1) % len(q.items)
		q.count--
		dropped = true
	}
	q.items[(q.head+q.count)%len(q.items)] = ask
	q.count++
	return shed, dropped
}

func (q *Queue) wake() {
	select {
	case q.signal <- struct{}{}:
	default:
	}
}

// Next blocks until an ask is available, the context ends, or the queue closes
// and drains. The second return is false only when no further ask will arrive.
func (q *Queue) Next(ctx context.Context) (Ask, bool) {
	for {
		q.mu.Lock()
		if q.count > 0 {
			ask := q.items[q.head]
			q.items[q.head] = Ask{}
			q.head = (q.head + 1) % len(q.items)
			q.count--
			q.mu.Unlock()
			return ask, true
		}
		q.mu.Unlock()
		select {
		case <-q.signal:
		case <-q.done:
			// Re-checked rather than returned, so a close during a push still
			// hands back the ask that push added.
			if q.Depth() == 0 {
				return Ask{}, false
			}
		case <-ctx.Done():
			return Ask{}, false
		}
	}
}

// Depth is what the coalescer's high-water comparison reads, so widening
// responds to the real backlog rather than to the window's own contents.
func (q *Queue) Depth() int {
	q.mu.Lock()
	defer q.mu.Unlock()
	return q.count
}

// Close stops further waiting once the buffer drains. Asks already queued stay
// readable, because shutting down is not a reason to drop admitted work.
func (q *Queue) Close() {
	q.mu.Lock()
	already := q.closed
	q.closed = true
	q.mu.Unlock()
	if !already {
		close(q.done)
	}
}

// Capacity is the bound, reported so a shed record can say what it was up
// against without the reader consulting a constant.
func (q *Queue) Capacity() int { return len(q.items) }

// TryNext pops without blocking. The coalescer drains with this and then waits
// on Signal, so one loop can watch the queue and a window deadline together.
func (q *Queue) TryNext() (Ask, bool) {
	q.mu.Lock()
	defer q.mu.Unlock()
	if q.count == 0 {
		return Ask{}, false
	}
	ask := q.items[q.head]
	q.items[q.head] = Ask{}
	q.head = (q.head + 1) % len(q.items)
	q.count--
	return ask, true
}

// Signal fires when an ask arrives. It carries one wakeup and assumes one
// reader, which the coalescer is: a second reader could miss a wakeup.
func (q *Queue) Signal() <-chan struct{} { return q.signal }

// Done closes when the queue does, so a reader learns that no further ask is
// coming without polling for it.
func (q *Queue) Done() <-chan struct{} { return q.done }
