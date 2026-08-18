package community

import "strings"

// One step owns every service-authored suffix and the budget they share. See
// docs/sirens-echo-tool-markup.md and sirens-echo#413.

// serviceFacts is what the service knows about the turn and the answer does
// not. See docs/sirens-echo-reply-assembly.md.
type serviceFacts struct {
	executed []ExecutedTool
	prefill  prefillNote
}

// serviceSuffix appends one piece of service-authored text. An appender renders
// its own content and never shortens the answer.
type serviceSuffix struct {
	name string

	append func(reply string, limit int, facts serviceFacts) string
}

// serviceSuffixOrder is the append order and the preference order at once,
// because the final cut reaches the tail first.
func serviceSuffixOrder() []serviceSuffix {
	return []serviceSuffix{
		{
			name: "issue references",
			append: func(reply string, limit int, facts serviceFacts) string {
				return appendIssueReferencesWithin(reply, limit, facts.executed)
			},
		},
		{
			// Ahead of the receipt, so a note about missing context is the last
			// thing cut rather than the first.
			name: "prefill truncation",
			append: func(reply string, _ int, facts serviceFacts) string {
				return appendPrefillNoteWithin(reply, unboundedReply, facts.prefill)
			},
		},
		{
			name: "tool disclosure",
			append: func(reply string, _ int, facts serviceFacts) string {
				// The footer is atomic, so the loop rather than the footer
				// decides what the answer can afford.
				return AppendToolDisclosureWithin(reply, unboundedReply, facts.executed...)
			},
		},
	}
}

const (

	// unboundedReply is what a transport with no ceiling declares, and what a
	// convergence pass renders against so the overflow stays visible.
	unboundedReply = 0
)

// AssembleReply appends every service-authored suffix inside one transport
// budget, shortening the answer rather than a suffix. Zero is unbounded.
func AssembleReply(answer string, limit int, executed ...ExecutedTool) string {
	return assembleReplyFacts(answer, limit, serviceFacts{executed: executed})
}

// assembleReplyFacts is the same step with every service fact, not only the
// tools. See docs/sirens-echo-reply-assembly.md.
func assembleReplyFacts(answer string, limit int, facts serviceFacts) string {
	return assembleReplyWithin(answer, limit, maxAssemblyPasses, facts)
}

// assembleReplyWithin converges rather than reserving once, because suffix
// length is not monotone in the answer.
func assembleReplyWithin(
	answer string, limit, passes int, facts serviceFacts,
) string {
	if limit <= 0 {
		return appendServiceSuffixes(answer, unboundedReply, facts)
	}
	fitted := answer
	for pass := 0; ; pass++ {
		// Rendered unbounded, because a suffix that trims itself to the limit
		// hides the overflow the answer is supposed to pay.
		candidate := appendServiceSuffixes(fitted, unboundedReply, facts)
		overflow := runeLen(candidate) - limit
		if overflow <= 0 {
			return candidate
		}
		if pass >= passes {
			break
		}
		room := runeLen(fitted) - overflow
		if room <= 0 {
			// The answer cannot pay, so it yields entirely and the suffixes
			// contend among themselves under the order above.
			fitted = ""
			break
		}
		fitted = strings.TrimRight(truncateRunes(fitted, room), " \t\n")
		if fitted == "" {
			break
		}
	}
	return withinBudget(fitted, limit, facts)
}

// withinBudget drops the least preferred suffix whole rather than cutting one
// into a half-rendered receipt. Defined rather than emergent.
func withinBudget(fitted string, limit int, facts serviceFacts) string {
	for count := len(serviceSuffixOrder()); count >= 0; count-- {
		candidate := appendFirstSuffixes(fitted, limit, facts, count)
		if runeLen(candidate) <= limit {
			return candidate
		}
	}
	// Only a limit shorter than the answer itself reaches here.
	return truncateRunes(fitted, limit)
}

// hasServiceSuffix reports whether the turn has service-authored text to
// append. Derived from the order above, so a new suffix needs no second edit.
func hasServiceSuffix(facts serviceFacts) bool {
	return appendServiceSuffixes("", unboundedReply, facts) != ""
}

func appendServiceSuffixes(
	reply string, limit int, facts serviceFacts,
) string {
	return appendFirstSuffixes(reply, limit, facts, len(serviceSuffixOrder()))
}

func appendFirstSuffixes(
	reply string, limit int, facts serviceFacts, count int,
) string {
	for index, suffix := range serviceSuffixOrder() {
		if index >= count {
			break
		}
		reply = suffix.append(reply, limit, facts)
	}
	return reply
}

func runeLen(value string) int { return len([]rune(value)) }

// appendServiceLine adds one service-authored line, shortening the answer
// rather than the line. A note that vanishes under load reads as no note.
func appendServiceLine(reply string, limit int, line string) string {
	if line == "" {
		return reply
	}
	trimmed := strings.TrimRight(reply, "\n")
	if trimmed == "" {
		return line
	}
	if limit > 0 {
		room := limit - runeLen(line) - 2
		if room < 0 {
			room = 0
		}
		trimmed = strings.TrimRight(truncateRunes(trimmed, room), "\n")
	}
	if trimmed == "" {
		return line
	}
	return trimmed + "\n\n" + line
}
