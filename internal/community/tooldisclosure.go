package community

import (
	"fmt"
	"strings"
)

// The receipt a reader can see. See docs/sirens-echo-tool-disclosure.md.

const (
	toolDisclosureGlyph = "🔨"
	toolEmptyNote       = " — no results"
)

// toolOutcomeGlyph is the status vocabulary, shared with the reaction set and
// the progress element rather than invented here. See sirens-echo#385.
func toolOutcomeGlyph(outcome ToolOutcome) string {
	switch outcome {
	case ToolOutcomeFailed:
		return "❌"
	case ToolOutcomeEmpty:
		return "📭"
	default:
		return "✅"
	}
}

// AppendToolDisclosure adds the footer naming what ran. Service-authored, so it
// is appended after the reply checks rather than passed through them.
func AppendToolDisclosure(reply string, executed ...ExecutedTool) string {
	footer := toolDisclosure(executed)
	if footer == "" {
		return reply
	}
	trimmed := strings.TrimRight(reply, "\n")
	if trimmed == "" {
		return footer
	}
	return trimmed + "\n\n" + footer
}

// toolDisclosure renders one line per call, or per consecutive run of the same
// tool at the same status. A refusal calls nothing and gets nothing.
func toolDisclosure(executed []ExecutedTool) string {
	var lines []string
	for index := 0; index < len(executed); {
		current := executed[index]
		// Consecutive only, and never across a status change. See
		// docs/sirens-echo-tool-disclosure.md.
		run := 1
		for index+run < len(executed) &&
			executed[index+run].Name == current.Name &&
			executed[index+run].Outcome == current.Outcome {
			run++
		}
		index += run
		lines = append(lines, toolDisclosureLine(current, run))
	}
	return strings.Join(lines, "\n")
}

func toolDisclosureLine(call ExecutedTool, run int) string {
	line := fmt.Sprintf(
		"> %s %s `%s`",
		toolDisclosureGlyph, toolOutcomeGlyph(call.Outcome), call.Name,
	)
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
