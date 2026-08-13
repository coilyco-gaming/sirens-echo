package community

import (
	"testing"
	"time"

	"github.com/modelcontextprotocol/go-sdk/mcp"
)

// A listing that never left the process and one that did are different events.
// Duration was doing the work of a field. See sirens-echo#520.

// The seam the span attribute reports. A fresh cache lists nothing, so a
// per-turn lookup is not a per-turn round trip.
func TestAFreshCacheNeedsNoListing(t *testing.T) {
	t.Parallel()
	now := time.Now()
	server := &supervisedServer{
		tools:     []*mcp.Tool{{Name: "eco.get_market"}},
		refreshed: now,
	}
	if server.needsTools(time.Hour, now) {
		t.Error("a cache refreshed now wanted a listing")
	}
	if !server.needsTools(time.Hour, now.Add(2*time.Hour)) {
		t.Error("a cache two hours past its interval did not want a listing")
	}
}

// A first listing and a list_changed notification are the two cases that must
// still reach the network, or the cache would serve a roster that moved.
func TestAFirstListingAndAChangedRosterStillList(t *testing.T) {
	t.Parallel()
	now := time.Now()
	if !(&supervisedServer{}).needsTools(time.Hour, now) {
		t.Error("a server that has never listed did not want a listing")
	}
	notified := &supervisedServer{
		tools:     []*mcp.Tool{{Name: "eco.get_market"}},
		refreshed: now,
	}
	notified.stale.Store(true)
	if !notified.needsTools(time.Hour, now) {
		t.Error("a roster that announced a change was served from cache")
	}
	// The notification is consumed, so one change costs one listing.
	if notified.needsTools(time.Hour, now) {
		t.Error("one list_changed produced a second listing")
	}
}

// A transport that can notify does not expire, because expiry exists only for
// one that cannot. This is what makes the steady state zero listings.
func TestANotifyingTransportDoesNotExpire(t *testing.T) {
	t.Parallel()
	now := time.Now()
	server := &supervisedServer{
		tools:     []*mcp.Tool{{Name: "eco.get_market"}},
		refreshed: now,
		notifies:  true,
	}
	if server.needsTools(time.Hour, now.Add(24*time.Hour)) {
		t.Error("a notifying transport expired, so it lists on a schedule it does not need")
	}
}
