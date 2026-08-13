package community

import (
	"context"
	"testing"
)

// Three explanations produced identical telemetry: a quiet guild, stopped
// ingress, and a dead process. See issue 190.

// Counts drain on each beat, so a reader sees the interval rather than a total
// that needs a baseline to interpret.
func TestABeatDrainsItsCounters(t *testing.T) {
	t.Parallel()
	beats := &heartbeat{}
	beats.observe()
	beats.observe()
	beats.admit()
	beats.reply()
	beats.beat(context.Background(), telemetryOrNoop(nil))
	if got := beats.observed.Load(); got != 0 {
		t.Errorf("observed did not drain: %d", got)
	}
	if got := beats.admitted.Load(); got != 0 {
		t.Errorf("admitted did not drain: %d", got)
	}
	if got := beats.replied.Load(); got != 0 {
		t.Errorf("replied did not drain: %d", got)
	}
}

// Messages arriving and none producing a turn is the shape nothing detects
// today, and it is the one this separates from a quiet guild.
func TestObservedCountsBeforeEligibility(t *testing.T) {
	t.Parallel()
	beats := &heartbeat{}
	beats.observe()
	if got := beats.observed.Load(); got != 1 {
		t.Fatalf("observed = %d, want 1", got)
	}
	if got := beats.admitted.Load(); got != 0 {
		t.Errorf("an ineligible message was counted as admitted: %d", got)
	}
}

// The HTTP-only deployment opens no gateway and has no heartbeat, so every
// counter has to be safe on a nil receiver rather than guarded at each call.
func TestANilHeartbeatIsInert(t *testing.T) {
	t.Parallel()
	var beats *heartbeat
	beats.observe()
	beats.admit()
	beats.reply()
}
