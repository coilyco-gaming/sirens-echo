package community

import "strings"

// One step owns every service-authored suffix and the budget they share. See
// docs/sirens-echo-tool-disclosure.md and sirens-echo#413.

// serviceSuffix appends one piece of service-authored text. An appender renders
// its own content and never shortens the answer.
type serviceSuffix struct {
	name string

	append func(reply string, limit int, executed []ExecutedTool) string
}

// serviceSuffixOrder is the append order and the preference order at once,
// because the final cut reaches the tail first.
func serviceSuffixOrder() []serviceSuffix {
	return []serviceSuffix{
		{
			name: "issue references",
			append: func(reply string, limit int, executed []ExecutedTool) string {
				return appendIssueReferencesWithin(reply, limit, executed)
			},
		},
		{
			name: "tool disclosure",
			append: func(reply string, _ int, executed []ExecutedTool) string {
				// The footer is atomic, so the loop rather than the footer
				// decides what the answer can afford.
				return AppendToolDisclosureWithin(reply, unboundedReply, executed...)
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
	return assembleReplyWithin(answer, limit, maxAssemblyPasses, executed)
}

// assembleReplyWithin converges rather than reserving once, because suffix
// length is not monotone in the answer.
func assembleReplyWithin(
	answer string, limit, passes int, executed []ExecutedTool,
) string {
	if limit <= 0 {
		return appendServiceSuffixes(answer, unboundedReply, executed)
	}
	fitted := answer
	for pass := 0; ; pass++ {
		// Rendered unbounded, because a suffix that trims itself to the limit
		// hides the overflow the answer is supposed to pay.
		candidate := appendServiceSuffixes(fitted, unboundedReply, executed)
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
	return withinBudget(fitted, limit, executed)
}

// withinBudget drops the least preferred suffix whole rather than cutting one
// into a half-rendered receipt. Defined rather than emergent.
func withinBudget(fitted string, limit int, executed []ExecutedTool) string {
	for count := len(serviceSuffixOrder()); count >= 0; count-- {
		candidate := appendFirstSuffixes(fitted, limit, executed, count)
		if runeLen(candidate) <= limit {
			return candidate
		}
	}
	// Only a limit shorter than the answer itself reaches here.
	return truncateRunes(fitted, limit)
}

func appendServiceSuffixes(
	reply string, limit int, executed []ExecutedTool,
) string {
	return appendFirstSuffixes(reply, limit, executed, len(serviceSuffixOrder()))
}

func appendFirstSuffixes(
	reply string, limit int, executed []ExecutedTool, count int,
) string {
	for index, suffix := range serviceSuffixOrder() {
		if index >= count {
			break
		}
		reply = suffix.append(reply, limit, executed)
	}
	return reply
}

func runeLen(value string) int { return len([]rune(value)) }
