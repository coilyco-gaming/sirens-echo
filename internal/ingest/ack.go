package ingest

import "context"

// Acknowledger makes one ask individually visible. Discord always has one, and
// a site changelog is an additional sink. See docs/sirens-echo-admission.md.
type Acknowledger interface {
	// Queued marks an ask as received. Called at ingress before the coalescer
	// can have seen it, so the mark never describes a batch.
	Queued(ctx context.Context, ask Ask) error
	// Shed retracts that mark when the bounded queue drops the ask. A queued
	// mark left on a shed ask is a promise the service will not keep.
	Shed(ctx context.Context, ask Ask) error
}

// Acknowledgers fans one ask out to every surface. A failing sink never stops
// the others, because the path already owes the member an answer.
type Acknowledgers []Acknowledger

// Queued marks the ask on every sink and reports the first failure, having
// already tried the rest.
func (a Acknowledgers) Queued(ctx context.Context, ask Ask) error {
	return a.each(func(sink Acknowledger) error { return sink.Queued(ctx, ask) })
}

// Shed retracts the mark on every sink under the same posture.
func (a Acknowledgers) Shed(ctx context.Context, ask Ask) error {
	return a.each(func(sink Acknowledger) error { return sink.Shed(ctx, ask) })
}

func (a Acknowledgers) each(apply func(Acknowledger) error) error {
	var first error
	for _, sink := range a {
		if sink == nil {
			continue
		}
		if err := apply(sink); err != nil && first == nil {
			first = err
		}
	}
	return first
}

// Silent acknowledges nothing. It is the HTTP caller's sink, which learns the
// same facts from its own response, and never a Discord deployment's.
type Silent struct{}

func (Silent) Queued(context.Context, Ask) error { return nil }
func (Silent) Shed(context.Context, Ask) error   { return nil }
