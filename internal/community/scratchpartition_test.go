package community

import (
	"strings"
	"testing"
)

// A partition name has to be flat, which TestScratchPartitionNameIsFlat holds,
// and injective, which nothing held. See docs/sirens-echo-scratchpad.md.

// partitionPair is two requesters that must not share a partition, and whether
// they currently do. A row where the two disagree is the open defect.
type partitionPair struct {
	left, right string
	collidesNow bool
	issue       string
}

// The HTTP requester is "http:" plus a caller-asserted header, so a collision
// is reachable on purpose rather than only by accident.
var partitionPairs = []partitionPair{
	{left: "http:fleet-client", right: "http:fleetclient", collidesNow: true, issue: "270"},
	{left: "http:fleet-client", right: "http:fleet_client", collidesNow: true, issue: "270"},
	{left: "http:fleet-client", right: "http:fleet.client", collidesNow: true, issue: "270"},
	{left: "http:ops", right: "http:o-p-s", collidesNow: true, issue: "270"},
	{left: "http:anonymous", right: "http:anon-ymous", collidesNow: true, issue: "270"},
	{left: "a/b", right: "ab", collidesNow: true, issue: "270"},

	// Discord author IDs are pure digits, so no two distinct users collide.
	{left: "318190481467244544", right: "318190481467244545", collidesNow: false},
	{left: "http:one", right: "http:two", collidesNow: false},
	{left: "318190481467244544", right: "http:318190481467244544", collidesNow: false},
}

// Two requesters sharing a partition share every spilled tool result in it, and
// the spill is automatic rather than something either of them asked for.
func TestScratchPartitionNameIsInjective(t *testing.T) {
	t.Parallel()
	for _, pair := range partitionPairs {
		collides := scratchPartitionName(pair.left) == scratchPartitionName(pair.right)
		if collides == pair.collidesNow {
			continue
		}
		if !pair.collidesNow {
			t.Errorf("regression: %q and %q now share partition %q",
				pair.left, pair.right, scratchPartitionName(pair.left))
			continue
		}
		t.Errorf("%q and %q no longer collide. If issue %s was fixed, set "+
			"collidesNow to false and clear the issue field", pair.left, pair.right, pair.issue)
	}
}

// Flatness and injectivity are separate properties, and a fix for one can break
// the other. Encoding a requester keeps both; deleting characters keeps one.
func TestScratchPartitionNameStaysFlatWhileInjective(t *testing.T) {
	t.Parallel()
	for _, pair := range partitionPairs {
		for _, requester := range []string{pair.left, pair.right} {
			name := scratchPartitionName(requester)
			if name == "" {
				t.Errorf("%q produced an empty partition name", requester)
			}
			if strings.ContainsAny(name, `/\. `) {
				t.Errorf("partition name %q from %q is not flat", name, requester)
			}
		}
	}
}
