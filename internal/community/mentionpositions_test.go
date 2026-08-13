package community

import (
	"strings"
	"testing"
)

// Five fixes reached this function and each was correct about the position it
// was shown. Enumerating positions is how the class is seen. See issue 486.

// mentionPosition is one token of one reply, hand-classified. Derived
// classification would use the implementation to grade itself.
type mentionPosition struct {
	reply   string
	name    string
	resolve bool
	why     string
}

// Every token of two replies a member actually receives, so a rule that fixes
// one position and moves the defect to the next is visible in one run.
var mentionPositions = []mentionPosition{
	// A link, every label and segment. urlSpan covers the whole span.
	{"See https://eco-app.coilysiren.me/jobs", "eco", false, "host label inside a link"},
	{"See https://eco-app.coilysiren.me/jobs", "app", false, "host label inside a link"},
	{"See https://eco-app.coilysiren.me/jobs", "coilysiren", false, "host label inside a link"},
	{"See https://eco-app.coilysiren.me/jobs", "jobs", false, "path segment inside a link"},

	// The same address with no scheme, which urlSpan does not cover. The dotted
	// labels are read as labels. The first one is issue 468's remaining row.
	{"See eco-app.coilysiren.me/jobs", "coilysiren", false, "dotted label, not a person"},
	{"See eco-app.coilysiren.me/jobs", "app", false, "dotted label, not a person"},

	// The receipt, which rides on every tool-using turn.
	{"x\n\n> 🔨 ✅ `eco.get_market`", "eco", false, "tool name in a code span"},
	{"x\n\n> 🔨 ✅ `eco.get_market`", "get_market", false, "tool name in a code span"},

	// A quoted command, inline and fenced.
	{"run `eco status` to check", "eco", false, "command in a code span"},
	{"run `eco status` to check", "status", false, "command in a code span"},
	{"```\neco --version\n```", "eco", false, "command in a fenced block"},

	// Discord markup, where the inner text is an id or an emoji name.
	{"nice <:eco:1234567890>", "eco", false, "emoji name inside markup"},
	{"ask in <#1499488069269590262>", "1499488069269590262", false, "channel id inside markup"},

	// Prose, which is the whole point. A name is still a name with emphasis,
	// quotation, or punctuation around it.
	{"eco confirmed it", "eco", true, "plain prose"},
	{"**eco** confirmed it", "eco", true, "emphasised prose"},
	{`she said "eco" earlier`, "eco", true, "quoted prose"},
	{"thanks eco.", "eco", true, "prose ending a sentence"},
	{"eco, can you look?", "eco", true, "prose before a comma"},
	{"(eco asked)", "eco", true, "prose inside brackets"},
}

// A rule that fixes one position and exposes the next fails here rather than in
// a member's reply, which is how the last five arrived.
func TestEveryPositionResolvesOrDoesNot(t *testing.T) {
	t.Parallel()
	for _, position := range mentionPositions {
		roster := mentionRoster{}
		roster.add(position.name, "999")
		out, resolved := roster.resolveMentions(position.reply)
		got := len(resolved) > 0
		if got != position.resolve {
			t.Errorf("%s: name %q in %q resolved=%v, want %v\n  got %q",
				position.why, position.name, position.reply, got, position.resolve, out)
			continue
		}
		// A name that must not resolve must also leave the text alone, since a
		// rewrite without a ping still breaks the address.
		if !position.resolve && out != position.reply {
			t.Errorf("%s: name %q rewrote text it must not touch:\n  want %q\n  got  %q",
				position.why, position.name, position.reply, out)
		}
		if position.resolve && !strings.Contains(out, "<@999>") {
			t.Errorf("%s: name %q resolved without rewriting: %q",
				position.why, position.name, out)
		}
	}
}
