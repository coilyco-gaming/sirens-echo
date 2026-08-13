package community

import (
	"strings"
	"testing"
)

// A bare host is not a link by urlSpan, so the span fix cannot reach it. The
// name rule can. See sirens-echo#481.

const hostMentionID = "1024000000000000001"

func hostRoster() mentionRoster {
	return mentionRoster{"coilysiren": hostMentionID}
}

// The reported residual. A hostname label is not a person, and rewriting it
// both breaks the address and sends a real ping.
func TestASchemelessHostnameIsNotAMention(t *testing.T) {
	t.Parallel()
	for _, reply := range []string{
		"See eco-app.coilysiren.me/jobs",
		"coilysiren.me/jobs has the list",
		"Both eco-app.coilysiren.me and forgejo.coilysiren.me are up",
		"Mail is at kai@eco-app.coilysiren.me today",
	} {
		got, resolved := hostRoster().resolveMentions(reply)
		if got != reply {
			t.Errorf("host rewritten:\n  in =%q\n  out=%q", reply, got)
		}
		if len(resolved) != 0 {
			t.Errorf("%q pinged %v for a hostname label", reply, resolved)
		}
	}
}

// The fix must not buy quiet hostnames by refusing punctuation. A trailing dot
// that ends a sentence is not a domain.
func TestANameEndingASentenceStillResolves(t *testing.T) {
	t.Parallel()
	for _, reply := range []string{
		"Thanks, coilysiren.",
		"That was coilysiren",
		"Ask coilysiren, who owns it",
		"Ask coilysiren. The page is stale.",
		"coilysiren",
	} {
		got, resolved := hostRoster().resolveMentions(reply)
		if !strings.Contains(got, "<@"+hostMentionID+">") {
			t.Errorf("a prose mention was refused:\n  in =%q\n  out=%q", reply, got)
		}
		if len(resolved) != 1 {
			t.Errorf("%q resolved %v, want exactly one", reply, resolved)
		}
	}
}

// The reason the resolver reads every occurrence rather than the first. A
// hostname first and a person second must still reach the person.
func TestAHostnameDoesNotConsumeTheLaterProseMention(t *testing.T) {
	t.Parallel()
	reply := "The page eco-app.coilysiren.me/jobs is stale, so ask coilysiren"
	got, resolved := hostRoster().resolveMentions(reply)

	if !strings.Contains(got, "eco-app.coilysiren.me/jobs") {
		t.Errorf("the hostname was rewritten:\n  %q", got)
	}
	if !strings.HasSuffix(got, "ask <@"+hostMentionID+">") {
		t.Errorf("the prose mention was not resolved:\n  %q", got)
	}
	if len(resolved) != 1 {
		t.Errorf("resolved %v, want exactly one", resolved)
	}
}

// The shapes pull request 472 already fixed stay fixed. This rule is additional
// to the link spans and does not replace them.
func TestEveryLinkShapeStaysUntouched(t *testing.T) {
	t.Parallel()
	for _, reply := range []string{
		"See https://eco-app.coilysiren.me/jobs.",
		"See https://eco-app.coilysiren.me for jobs",
		"See [jobs](https://eco-app.coilysiren.me/jobs)",
		"See <https://eco-app.coilysiren.me/jobs>",
	} {
		if got, _ := hostRoster().resolveMentions(reply); got != reply {
			t.Errorf("link rewritten:\n  in =%q\n  out=%q", reply, got)
		}
	}
}

// The boundary itself, stated as a table so the punctuation case and the
// domain case are visibly different rules rather than one fuzzy one.
func TestInDottedIdentifierSeparatesDomainsFromPunctuation(t *testing.T) {
	t.Parallel()
	for _, testCase := range []struct {
		name  string
		text  string
		start int
		end   int
		want  bool
	}{
		{"leading dot", "a.name", 2, 6, true},
		{"trailing dot then letter", "name.me", 0, 4, true},
		{"trailing dot then digit", "name.4chan", 0, 4, true},
		{"trailing dot then space", "name. Next", 0, 4, false},
		{"trailing dot at the end", "name.", 0, 4, false},
		{"no dots", "name here", 0, 4, false},
		{"trailing dot then multibyte letter", "name.école", 0, 4, true},
	} {
		got := inDottedIdentifier(testCase.text, testCase.start, testCase.end)
		if got != testCase.want {
			t.Errorf("%s: %q -> %v, want %v",
				testCase.name, testCase.text, got, testCase.want)
		}
	}
}

// The first label of a schemeless host is where a space precedes the name, so
// nothing before it says hostname. See sirens-echo#515.
func TestTheFirstLabelOfAHostIsNotAPerson(t *testing.T) {
	t.Parallel()
	for _, reply := range []string{
		"See eco-app.coilysiren.me/jobs",
		"see eco-app.coilysiren.me for jobs",
		"eco-app.coilysiren.me is the page",
		"try eco-app-two.example.com today",
	} {
		roster := mentionRoster{"eco": hostMentionID}
		got, resolved := roster.resolveMentions(reply)
		if got != reply {
			t.Errorf("the host was rewritten:\n  in =%q\n  out=%q", reply, got)
		}
		if len(resolved) != 0 {
			t.Errorf("%q pinged %v for the first label of a host", reply, resolved)
		}
	}
}

// Every other position in the same address, so the fix is not read as covering
// only the one Quail reported.
func TestEveryLabelOfTheSameHostIsSafe(t *testing.T) {
	t.Parallel()
	const reply = "See eco-app.coilysiren.me/jobs"
	for _, name := range []string{"eco", "app", "coilysiren", "jobs"} {
		roster := mentionRoster{name: hostMentionID}
		got, resolved := roster.resolveMentions(reply)
		if got != reply || len(resolved) != 0 {
			t.Errorf("name %q broke the address: %q %v", name, got, resolved)
		}
	}
}

// The case that keeps the rule honest. A hyphen leading to a word is prose, so
// a member named eco is still reached in eco-friendly.
func TestAHyphenatedWordInProseStillResolves(t *testing.T) {
	t.Parallel()
	for _, reply := range []string{
		"eco-friendly builds are cheaper",
		"ask eco-friendly about it",
		"eco replied already",
	} {
		roster := mentionRoster{"eco": hostMentionID}
		got, resolved := roster.resolveMentions(reply)
		if len(resolved) != 1 {
			t.Errorf("a prose name stopped resolving: %q -> %q %v",
				reply, got, resolved)
		}
	}
}

// The walk itself, stated as a table so the two directions are visibly
// different rules rather than one fuzzy one.
func TestReachesADottedLabelSeparatesHostsFromWords(t *testing.T) {
	t.Parallel()
	for _, testCase := range []struct {
		rest string
		want bool
	}{
		{"-app.coilysiren.me/jobs", true},
		{".coilysiren.me", true},
		{"-app-two.example.com", true},
		{"-friendly builds", false},
		{"-friendly", false},
		{" replied", false},
		{".", false},
		{". Next", false},
		{"", false},
		{"-app/jobs", false},
	} {
		if got := reachesADottedLabel(testCase.rest); got != testCase.want {
			t.Errorf("%q -> %v, want %v", testCase.rest, got, testCase.want)
		}
	}
}
