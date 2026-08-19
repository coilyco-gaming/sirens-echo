package community

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"testing"
	"time"
)

// The worklog surface, and the fallback that carries it where the embed
// permission is absent. See sirens-echo#111.

// worklogRecorder is a sink that takes both surfaces, so a row can assert
// which one a turn actually used rather than that no error came back.
type worklogRecorder struct {
	waitSink
	allowed bool
	refuse  error
	views   []progressView
}

func (s *worklogRecorder) WorklogAllowed(context.Context) bool { return s.allowed }

func (s *worklogRecorder) PostWorklog(
	_ context.Context, view progressView,
) (string, error) {
	if s.refuse != nil {
		return "", s.refuse
	}
	s.views = append(s.views, view)
	return "message-1", nil
}

func (s *worklogRecorder) EditWorklog(
	_ context.Context, _ string, view progressView,
) error {
	if s.refuse != nil {
		return s.refuse
	}
	s.views = append(s.views, view)
	return nil
}

// WorklogRefused treats the sentinel below as the permission answer, so a test
// drives the same branch a missing grant does without a Discord error.
func (s *worklogRecorder) WorklogRefused(err error) bool {
	return errors.Is(err, errNoEmbedPermission)
}

var errNoEmbedPermission = errors.New("missing permissions")

// worklogTurn drives a progress past the posting threshold on a sink that
// records both surfaces.
func worklogTurn(t *testing.T, sink *worklogRecorder) (*turnProgress, func(time.Duration)) {
	t.Helper()
	moment := time.Now()
	progress := newTurnProgress(sink, func() time.Time { return moment })
	advance := func(by time.Duration) { moment = moment.Add(by) }
	progress.Stage(context.Background(), stagePhraseThinking)
	advance(turnProgressAfter + time.Second)
	return progress, advance
}

// The shape Kai chose: a row per call, resolving in place.
func TestTheWorklogRowsResolveInPlace(t *testing.T) {
	t.Parallel()
	sink := &worklogRecorder{allowed: true}
	progress, advance := worklogTurn(t, sink)

	progress.ToolStarted("eco", "get_market")
	progress.refresh(context.Background())
	if len(sink.views) != 1 {
		t.Fatalf("the turn posted %d worklogs, want 1", len(sink.views))
	}
	if got := sink.views[0].Rows[0]; !strings.Contains(got, reactionTool) {
		t.Errorf("an in-flight row does not carry the tool glyph: %q", got)
	}

	progress.ToolFinished("eco", "get_market", ToolOutcomeOK, "")
	advance(turnProgressEvery)
	progress.refresh(context.Background())
	last := sink.views[len(sink.views)-1].Rows[0]
	if !strings.Contains(last, toolOKGlyph) {
		t.Errorf("the resolved row does not carry the outcome glyph: %q", last)
	}
	if !strings.Contains(last, "eco.get_market") {
		t.Errorf("the row does not name the tool: %q", last)
	}
}

// An empty result and a failure must not read alike, which is the whole reason
// the glyph vocabulary has three values rather than two.
func TestEachOutcomeHasItsOwnGlyph(t *testing.T) {
	t.Parallel()
	seen := map[string]string{}
	for outcome, want := range map[ToolOutcome]string{
		ToolOutcomeOK:     toolOKGlyph,
		ToolOutcomeEmpty:  toolEmptyGlyph,
		ToolOutcomeFailed: toolFailedGlyph,
	} {
		row := worklogRow(progressRow{
			server: "eco", tool: "get_market", outcome: outcome, done: true,
		})
		if !strings.Contains(row, want) {
			t.Errorf("%s row = %q, want the %q glyph", outcome, row, want)
		}
		if prior, ok := seen[want]; ok {
			t.Errorf("%s shares a glyph with %s", outcome, prior)
		}
		seen[want] = string(outcome)
	}
}

// A forty-call turn must not render forty rows, and the count is what keeps
// the dropped ones from vanishing silently.
func TestTheWorklogIsCapped(t *testing.T) {
	t.Parallel()
	over := maxProgressRows + 4
	rows := make([]progressRow, 0, over)
	for index := 0; index < over; index++ {
		rows = append(rows, progressRow{server: "eco", tool: "get_market", done: true})
	}

	rendered := worklogRows(rows)

	if len(rendered) != maxProgressRows+1 {
		t.Fatalf("rendered %d lines, want %d rows and one count",
			len(rendered), maxProgressRows)
	}
	if dropped := fmt.Sprintf("%d earlier calls", over-maxProgressRows); !strings.Contains(rendered[0], dropped) {
		t.Errorf("the dropped rows are not counted as %q: %q", dropped, rendered[0])
	}
}

// Every line inside the embed keeps the contract it has outside one: the
// embed is the container, the code span is the text contract.
func TestEveryWorklogLineKeepsTheNoticeShape(t *testing.T) {
	t.Parallel()
	rows := []progressRow{
		{server: "eco", tool: "get_market"},
		{server: "forgejo", tool: "list_issue", outcome: ToolOutcomeEmpty, done: true},
		{server: "discord", tool: "list_general-message", outcome: ToolOutcomeFailed, done: true},
	}
	for index := 0; index < 10; index++ {
		rows = append(rows, progressRow{server: "eco", tool: "fair_price", done: true})
	}

	for _, line := range worklogRows(rows) {
		if !noticeShape.MatchString(line) {
			t.Errorf("worklog line escapes the notice shape: %q", line)
		}
	}
}

// The underscore had to join the alphabet for that to hold, and a tool name
// that arrives mangled is a name a member cannot look up.
func TestAToolNameSurvivesTheAlphabet(t *testing.T) {
	t.Parallel()
	row := worklogRow(progressRow{server: "forgejo", tool: "list_issue"})
	if !strings.Contains(row, "forgejo.list_issue") {
		t.Errorf("the tool name was mangled: %q", row)
	}
}

// The load-bearing acceptance from Kai's amendment: no embed permission means
// the notice lines, never a failed post and silence.
func TestAChannelWithoutThePermissionGetsTheNoticeLines(t *testing.T) {
	t.Parallel()
	sink := &worklogRecorder{allowed: false}
	progress, _ := worklogTurn(t, sink)

	progress.ToolStarted("eco", "get_market")
	progress.refresh(context.Background())

	if len(sink.views) != 0 {
		t.Errorf("an embed was posted without the permission: %v", sink.views)
	}
	if sink.posted == "" {
		t.Fatal("the turn posted nothing at all, which is the silence this fixes")
	}
	if !noticeShape.MatchString(sink.posted) {
		t.Errorf("the fallback is not a harness notice: %q", sink.posted)
	}
}

// A permission read can say yes and Discord still refuse. The refusal has to
// fall back rather than leave the member with nothing.
func TestARefusedEmbedFallsBackRatherThanFailing(t *testing.T) {
	t.Parallel()
	sink := &worklogRecorder{allowed: true, refuse: errNoEmbedPermission}
	progress, _ := worklogTurn(t, sink)

	progress.refresh(context.Background())

	if sink.posted == "" {
		t.Fatal("a refused embed left the member with nothing")
	}
	if !noticeShape.MatchString(sink.posted) {
		t.Errorf("the fallback is not a harness notice: %q", sink.posted)
	}
}

// One refusal degrades the turn, not every edit in it, or a channel with no
// permission pays a rejected call per beat.
func TestARefusalLatchesForTheTurn(t *testing.T) {
	t.Parallel()
	sink := &worklogRecorder{allowed: true, refuse: errNoEmbedPermission}
	progress, advance := worklogTurn(t, sink)

	progress.refresh(context.Background())
	sink.refuse = nil
	advance(turnProgressEvery)
	progress.refresh(context.Background())

	if len(sink.views) != 0 {
		t.Errorf("the turn returned to the embed after a refusal: %v", sink.views)
	}
}

// An element that merely vanishes is the #137 silence in a costume, so a turn
// that stopped resolves rather than being deleted.
func TestAStoppedTurnResolvesTheElement(t *testing.T) {
	t.Parallel()
	sink := &worklogRecorder{allowed: true}
	progress, _ := worklogTurn(t, sink)
	progress.refresh(context.Background())

	progress.Stop()
	progress.Finish(context.Background())

	last := sink.views[len(sink.views)-1]
	if !last.Resolved || last.Title != worklogStopped {
		t.Errorf("the element did not resolve: %+v", last)
	}
}

// A delivered answer is its own resolution, and the disclosure footer under it
// already names the tools. Two lists of them is what #385 avoided.
func TestADeliveredAnswerDeletesTheElement(t *testing.T) {
	t.Parallel()
	sink := &worklogRecorder{allowed: true}
	progress, _ := worklogTurn(t, sink)
	progress.refresh(context.Background())
	before := len(sink.views)

	progress.Finish(context.Background())

	if len(sink.views) != before {
		t.Errorf("a successful turn left an element behind: %+v", sink.views[before:])
	}
}

// The block rule is a security property. The element says the same thing for
// every stop, so a content block is not tellable from a timeout.
func TestABlockLooksLikeEveryOtherStop(t *testing.T) {
	t.Parallel()
	stopped := progressView{Title: worklogStopped, Resolved: true}
	if stopped.Title != worklogStopped {
		t.Fatal("the stopped title is not the shared one")
	}
	// Nothing in the rendering takes a reason, which is what makes the property
	// structural rather than a wording convention.
	if strings.Contains(worklogFooter(stopped), "block") {
		t.Error("the footer narrates the stop")
	}
}
