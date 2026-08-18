// Package coalesce batches related asks into one agent turn and drains those
// batches across a worker pool.
//
// The lane it replaces served every ask on one execution slot, which at two
// arrivals a minute against a 33 second turn is a utilization above one: an
// unbounded queue that falls behind on the first slow turn and never recovers.
// Coalescing lowers the turn rate and the pool raises the service rate, and
// only both together buy headroom. See docs/sirens-echo-admission.md.
package coalesce

import "time"

// Named local defaults. Every one of these is a knob that does not exist yet.
// TODO(prong-c): move the set to config.go, whose knobs Prong C owns.
const (
	// DefaultWindow and DefaultBatch are the narrow window: a member's rapid
	// follow-ups land in one turn without a first ask waiting on a slow one.
	DefaultWindow = 25 * time.Second
	DefaultBatch  = 4
	// DefaultWideWindow and DefaultWideBatch coalesce harder once the backlog
	// says the narrow window is not keeping up.
	DefaultWideWindow = 45 * time.Second
	DefaultWideBatch  = 8
	// DefaultHighWater is the pool's own capacity, three workers holding a
	// narrow batch each. Past it the backlog is one the window is not draining.
	DefaultHighWater = DefaultWorkers * DefaultBatch
	// DefaultAgeCap is the oldest an ask may be before it forces the window
	// shut. It bounds waiting, so it is deliberately well above the wide window.
	DefaultAgeCap = 90 * time.Second
	// DefaultWorkers drain batches concurrently. Three is what the arithmetic
	// in the package comment needs, not a guess.
	DefaultWorkers = 3
	// DefaultDeadline is the hard bound on one turn, matching the p99 target.
	DefaultDeadline = 30 * time.Second
)

// Policy is the coalescer's tuning, resolved once so a zero value is usable.
type Policy struct {
	Window     time.Duration
	Batch      int
	WideWindow time.Duration
	WideBatch  int
	HighWater  int
	AgeCap     time.Duration
	Workers    int
	Deadline   time.Duration
}

// DefaultPolicy is the packaged tuning.
func DefaultPolicy() Policy {
	return Policy{
		Window:     DefaultWindow,
		Batch:      DefaultBatch,
		WideWindow: DefaultWideWindow,
		WideBatch:  DefaultWideBatch,
		HighWater:  DefaultHighWater,
		AgeCap:     DefaultAgeCap,
		Workers:    DefaultWorkers,
		Deadline:   DefaultDeadline,
	}
}

// withDefaults fills anything the caller left at zero, so a partly-specified
// policy cannot produce a window that never closes or a pool with no workers.
func (p Policy) withDefaults() Policy {
	base := DefaultPolicy()
	if p.Window <= 0 {
		p.Window = base.Window
	}
	if p.Batch <= 0 {
		p.Batch = base.Batch
	}
	if p.WideWindow <= 0 {
		p.WideWindow = base.WideWindow
	}
	if p.WideBatch <= 0 {
		p.WideBatch = base.WideBatch
	}
	if p.HighWater <= 0 {
		p.HighWater = base.HighWater
	}
	if p.AgeCap <= 0 {
		p.AgeCap = base.AgeCap
	}
	if p.Workers <= 0 {
		p.Workers = base.Workers
	}
	if p.Deadline <= 0 {
		p.Deadline = base.Deadline
	}
	return p
}

// window and batch are read on every window open, so widening responds to the
// backlog as it stands rather than to what it was when the window opened.
func (p Policy) window(depth int) time.Duration {
	if depth > p.HighWater {
		return p.WideWindow
	}
	return p.Window
}

func (p Policy) batch(depth int) int {
	if depth > p.HighWater {
		return p.WideBatch
	}
	return p.Batch
}
