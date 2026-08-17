package community

import (
	"fmt"
	"time"
)

// The worklog a long turn shows: one row per tool call, resolving in place.
// See docs/sirens-echo-worklog.md.

const (
	// worklogTitle is the running state. A member reads it as the turn being
	// alive rather than as an answer.
	worklogTitle = "Working on it"
	// worklogStopped is every terminal state that is not a delivered answer.
	// One wording for all of them. See docs/sirens-echo-worklog.md.
	worklogStopped = "Did not finish"
)

// progressRow is one tool call. Outcome is meaningless until done, which is
// what separates a call in flight from one that returned nothing.
type progressRow struct {
	server  string
	tool    string
	outcome ToolOutcome
	done    bool
}

// progressView is one rendering of the worklog, already reduced to text so a
// sink chooses a container rather than composing a message.
type progressView struct {
	Title    string
	Rows     []string
	Elapsed  time.Duration
	Tools    int
	Resolved bool
}

// worklogRow renders one call in the notice shape, so harness text inside an
// embed keeps the contract it has outside one. See docs/sirens-echo-delivery.md.
func worklogRow(row progressRow) string {
	glyph := reactionTool
	if row.done {
		glyph = toolOutcomeGlyph(row.outcome)
	}
	return stageNotice(glyph, row.server+"."+row.tool, false)
}

// worklogRows renders the visible rows, newest last, with a count standing in
// for whatever the cap dropped.
func worklogRows(rows []progressRow) []string {
	visible := rows
	dropped := 0
	if len(rows) > maxWorklogRows {
		dropped = len(rows) - maxWorklogRows
		visible = rows[dropped:]
	}
	rendered := make([]string, 0, len(visible)+1)
	if dropped > 0 {
		rendered = append(rendered, stageNotice(
			"", fmt.Sprintf("%d earlier calls", dropped), false))
	}
	for _, row := range visible {
		rendered = append(rendered, worklogRow(row))
	}
	return rendered
}

// worklogFooter is the proof of forward motion. A row count alone cannot say
// whether a turn with one slow call is working.
func worklogFooter(view progressView) string {
	seconds := int(view.Elapsed.Round(time.Second).Seconds())
	if view.Tools == 0 {
		return fmt.Sprintf("%d seconds elapsed", seconds)
	}
	return fmt.Sprintf("%d tools, %d seconds elapsed", view.Tools, seconds)
}
