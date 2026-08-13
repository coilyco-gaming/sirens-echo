package community

import "testing"

// sirens-echo#680 excludes an empty prefix, a leading dot and a doubled dot by
// name. Every other invalid label shape still matches. See sirens-echo#726.

// labelRow is a host and what hostAllowed does with it today against what it
// ought to do. A row where the two disagree is an open defect, not a target.
type labelRow struct {
	host string
	// allowedNow is the behaviour on origin/main. The test asserts this, so CI
	// stays honest about what ships rather than about what is wanted.
	allowedNow bool
	// shouldAllow is the intended behaviour. Where it differs from allowedNow
	// the row names the issue that closes the gap.
	shouldAllow bool
	issue       string
	why         string
}

// mustAllow are real subdomains the guard has to keep matching. This half is
// where a tightened prefix check regresses, so it is the half worth having.
var mustAllow = []labelRow{
	{host: "a.mozilla.com", allowedNow: true, shouldAllow: true, why: "one real label"},
	{host: "www.mozilla.com", allowedNow: true, shouldAllow: true, why: "the ordinary case"},
	{host: "deep.sub.mozilla.com", allowedNow: true, shouldAllow: true, why: "two real labels"},
	{host: "WWW.MOZILLA.COM", allowedNow: true, shouldAllow: true, why: "normalised inside"},
	{
		// Punycode is how a non-ASCII name is spelled on the wire, so it is a
		// valid label and must survive a rule written against non-ASCII.
		host: "xn--a.mozilla.com", allowedNow: true, shouldAllow: true,
		why: "punycode is a valid label",
	},
}

// mustRefuse are hosts no resolver would accept. RFC 1123 allows letters,
// digits and interior hyphens, so every row below is outside it.
var mustRefuse = []labelRow{
	{
		host: ".mozilla.com", allowedNow: false, shouldAllow: false,
		why: "empty label, closed by sirens-echo#668",
	},
	{
		host: "..mozilla.com", allowedNow: false, shouldAllow: false,
		why: "doubled dot, closed by sirens-echo#680",
	},
	{
		host: "a..b.mozilla.com", allowedNow: false, shouldAllow: false,
		why: "interior doubled dot",
	},
	{
		// A hyphen cannot open or close a label. This is the shape the fourth
		// acceptance row on 674 named and the merged guard still admits.
		host: "-.mozilla.com", allowedNow: false, shouldAllow: false,
		why: "a lone hyphen is not a label",
	},
	{
		host: "-a.mozilla.com", allowedNow: false, shouldAllow: false,
		why: "leading hyphen",
	},
	{
		host: "a-.mozilla.com", allowedNow: false, shouldAllow: false,
		why: "trailing hyphen",
	},
	{
		host: "_.mozilla.com", allowedNow: false, shouldAllow: false,
		why: "underscore is not a hostname character",
	},
	{
		host: "a b.mozilla.com", allowedNow: false, shouldAllow: false,
		why: "an interior space",
	},
	{
		// hostAllowed is documented as a predicate that must not depend on its
		// caller. A slash inside a host means the string is not a host.
		host: "a/b.mozilla.com", allowedNow: false, shouldAllow: false,
		why: "a path separator inside a host",
	},
	{
		host: "a:80.mozilla.com", allowedNow: false, shouldAllow: false,
		why: "a port separator inside a host",
	},
	{
		// A pattern holding a star is refused as a typo. A host holding one is
		// not, so the two ends disagree about what a star means.
		host: "*.mozilla.com", allowedNow: false, shouldAllow: false,
		why: "a star is not a label",
	},
	{
		host:       "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa.mozilla.com",
		allowedNow: false, shouldAllow: false,
		why: "64 octets, one over the limit",
	},
}

func runLabelRows(t *testing.T, rows []labelRow) {
	t.Helper()
	for _, row := range rows {
		allowed := hostAllowed(row.host, "*.mozilla.com")
		if allowed == row.allowedNow {
			continue
		}
		if row.allowedNow == row.shouldAllow {
			t.Errorf("regression on %q (%s): allowed = %v, want %v",
				row.host, row.why, allowed, row.allowedNow)
			continue
		}
		t.Errorf("behaviour changed on %q (%s): allowed = %v, was %v. If issue %s "+
			"was fixed, set allowedNow to %v and clear the issue field",
			row.host, row.why, allowed, row.allowedNow, row.issue, allowed)
	}
}

// The must-allow half. A prefix rule written to close the rows below regresses
// here first, which is why these are pinned beside them.
func TestARealSubdomainKeepsMatching(t *testing.T) {
	t.Parallel()
	runLabelRows(t, mustAllow)
}

// The must-refuse half, asserted against what ships rather than what is wanted.
// Three rows are closed and the rest name sirens-echo#726.
func TestAnInvalidLabelShape(t *testing.T) {
	t.Parallel()
	runLabelRows(t, mustRefuse)
}

// validHostLabel carries no uppercase arm because hostAllowed lowercases
// first. Reordering those two refuses every capitalised host, silently.
func TestALabelCheckThatRunsAfterLowercasing(t *testing.T) {
	t.Parallel()
	// The guarantee, from the caller's side: a capitalised host still matches.
	for _, host := range []string{
		"WWW.MOZILLA.COM",
		"Www.Mozilla.Com",
		"XN--A.mozilla.com",
	} {
		if !hostAllowed(host, "*.mozilla.com") {
			t.Errorf("%q was refused, so the label check now runs before "+
				"lowercasing and every capitalised host is rejected", host)
		}
	}
	// And the predicate underneath, so the failure names which half moved.
	if validHostLabel("WWW") {
		t.Error("validHostLabel gained an uppercase arm; the comment on it and " +
			"the ordering it relies on both need revisiting")
	}
	if !validHostLabel("www") {
		t.Error("validHostLabel stopped accepting a plain lowercase label")
	}
}

// The open rows are reported as one summary so the count is visible without
// reading the table. Use `go test -v` to see it.
func TestTheLabelShapeCorpusReportsOpenRows(t *testing.T) {
	t.Parallel()
	open := 0
	for _, row := range append(append([]labelRow{}, mustAllow...), mustRefuse...) {
		if row.allowedNow == row.shouldAllow {
			continue
		}
		open++
		t.Logf("issue %s, admitted: %q (%s)", row.issue, row.host, row.why)
	}
	t.Logf("%d rows still disagree with intended behaviour", open)
}
