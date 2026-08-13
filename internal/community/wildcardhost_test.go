package community

import "testing"

// A wildcard that is a suffix test accepts mozilla.com.evil.example. The dot is
// what stops it. See sirens-echo#663.

func TestAWildcardMatchesSubdomains(t *testing.T) {
	t.Parallel()
	for _, host := range []string{
		"www.mozilla.com",
		"developer.mozilla.com",
		"a.b.mozilla.com",
	} {
		if !hostAllowed(host, "*.mozilla.com") {
			t.Errorf("%s did not match *.mozilla.com", host)
		}
	}
}

// The attack the exact-match rule was written against, now the wildcard's to
// refuse. Every one of these ends in the pattern as a substring.
func TestAWildcardIsNotASuffixTest(t *testing.T) {
	t.Parallel()
	for name, host := range map[string]string{
		"registered lookalike": "mozilla.com.evil.example",
		"no separating dot":    "notmozilla.com",
		"glued label":          "evilmozilla.com",
		"pattern inside":       "x.mozilla.com.attacker.net",
	} {
		if hostAllowed(host, "*.mozilla.com") {
			t.Errorf("%s (%s) matched *.mozilla.com", name, host)
		}
	}
}

// The apex is a separate entry, per the issue: the bare form covers the domain
// itself and the wildcard covers subdomains.
func TestTheWildcardDoesNotCoverTheApex(t *testing.T) {
	t.Parallel()
	if hostAllowed("mozilla.com", "*.mozilla.com") {
		t.Error("*.mozilla.com matched the apex, which the issue lists separately")
	}
	if !hostAllowed("mozilla.com", "mozilla.com") {
		t.Error("the bare entry stopped matching its own host")
	}
}

// A bare entry must not gain subdomain reach, or every existing allowlist
// silently widens the moment this ships.
func TestABareEntryStillMatchesOnlyItself(t *testing.T) {
	t.Parallel()
	for _, host := range []string{"www.mozilla.com", "a.mozilla.com"} {
		if hostAllowed(host, "mozilla.com") {
			t.Errorf("%s matched the bare entry mozilla.com", host)
		}
	}
}

// A typo must refuse rather than match everything, which is the failure this
// repository has hit repeatedly.
func TestAMalformedPatternRefuses(t *testing.T) {
	t.Parallel()
	for name, pattern := range map[string]string{
		"bare star":   "*.",
		"star alone":  "*",
		"double star": "*.*.mozilla.com",
		"empty":       "",
		"whitespace":  "   ",
		"inner star":  "*.moz*lla.com",
	} {
		for _, host := range []string{"mozilla.com", "www.mozilla.com", "anything.example"} {
			if hostAllowed(host, pattern) {
				t.Errorf("%s (%q) matched %s", name, pattern, host)
			}
		}
	}
}

// Case and surrounding space in a configured entry must not change the answer.
func TestAnEntryIsNormalised(t *testing.T) {
	t.Parallel()
	if !hostAllowed("www.mozilla.com", "  *.MOZILLA.com  ") {
		t.Error("a padded uppercase wildcard did not match")
	}
	if !hostAllowed("mozilla.com", " MOZILLA.COM ") {
		t.Error("a padded uppercase bare entry did not match")
	}
}
