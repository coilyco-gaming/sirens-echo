package community

import (
	"fmt"
	"strings"
)

// The receipt a reader can see. See docs/sirens-echo-tool-markup.md.

const (
	// The two shared with the reaction set are references, not copies, so the
	// surfaces cannot drift apart. See docs/sirens-echo-tool-markup.md.
	toolDisclosureGlyph = reactionTool
	toolFailedGlyph     = reactionFailed
	// These two have no reaction counterpart and are defined here, as escapes
	// for the same reason sirens-echo#447 escaped the reactions.
	toolOKGlyph    = "\u2705"
	toolEmptyGlyph = "\U0001F4ED"
	toolEmptyNote  = " — no results"
	// skillReadGlyph marks a row naming the reference a skill read delivered,
	// on both member surfaces. See docs/sirens-echo-worklog.md.
	skillReadGlyph = "\U0001F4D6"
)

// toolOutcomeGlyph is the status vocabulary, shared with the reaction set and
// the progress element rather than invented here. See sirens-echo#385.
func toolOutcomeGlyph(outcome ToolOutcome) string {
	switch outcome {
	case ToolOutcomeFailed:
		return toolFailedGlyph
	case ToolOutcomeEmpty:
		return toolEmptyGlyph
	default:
		return toolOKGlyph
	}
}

// AppendToolDisclosure adds the footer naming what ran, unbounded. Service
// authored, so it is appended after the reply checks rather than through them.
func AppendToolDisclosure(reply string, executed ...ExecutedTool) string {
	return AppendToolDisclosureWithin(reply, 0, executed...)
}

// AppendToolDisclosureWithin keeps the footer inside a transport's send budget
// by shortening the answer. A limit of zero is unbounded.
func AppendToolDisclosureWithin(
	reply string, limit int, executed ...ExecutedTool,
) string {
	return appendServiceLine(reply, limit, toolDisclosure(executed))
}

// toolDisclosure renders one line per call, or per consecutive run of the same
// tool at the same status. A refusal calls nothing and gets nothing.
func toolDisclosure(executed []ExecutedTool) string {
	var lines []string
	for index := 0; index < len(executed); {
		current := executed[index]
		// Consecutive only, and never across a status change. See
		// docs/sirens-echo-tool-markup.md.
		run := 1
		for index+run < len(executed) &&
			executed[index+run].Name == current.Name &&
			executed[index+run].Outcome == current.Outcome &&
			executed[index+run].Detail == current.Detail {
			run++
		}
		index += run
		lines = append(lines, toolDisclosureLine(current, run))
	}
	return strings.Join(lines, "\n")
}

func toolDisclosureLine(call ExecutedTool, run int) string {
	glyphs := toolDisclosureGlyph + " " + toolOutcomeGlyph(call.Outcome)
	label := call.Label()
	if call.Detail != "" {
		glyphs += " " + skillReadGlyph
		label = noticeBody(call.Detail)
	}
	line := fmt.Sprintf("> %s `%s`", glyphs, label)
	if run > 1 {
		line += fmt.Sprintf(" ×%d", run)
	}
	// The glyph is a scanning anchor and not the message, so a reader who does
	// not recognise it still gets the fact. See sirens-echo#203.
	if call.Outcome == ToolOutcomeEmpty {
		line += toolEmptyNote
	}
	return line
}
