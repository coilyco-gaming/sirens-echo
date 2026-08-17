package community

import (
	"context"
	"fmt"
	"log/slog"
	"strings"
	"sync"
	"time"
)

// A long turn says what it is doing. A short one is untouched, because a
// progress line for a two-second reply is noise. See docs/sirens-echo-progress.md.

// The stage phrases. They come from the closed notice vocabulary, so a progress
// line reads as the harness speaking rather than as the model.
const (
	stagePhraseHistory  = "reading recent messages"
	stagePhraseThinking = "thinking"
	stagePhraseTool     = "calling a tool"
	stagePhraseChecking = "checking the reply"
)

// stageDecoration is what a stage line wears. The icon and the ellipsis are
// separate fields because Kai named them separately. See sirens-echo#370.
type stageDecoration struct {
	icon     string
	trailing bool
}

// stageDecorations holds the stages Kai has named. An unlisted stage renders
// plain and unterminated, which is the standing answer for the other three.
var stageDecorations = map[string]stageDecoration{
	stagePhraseThinking: {icon: "\U0001F914", trailing: true},
}

// stageLine renders one stage in whatever it wears.
func stageLine(phrase string) string {
	decoration := stageDecorations[phrase]
	return stageNotice(decoration.icon, phrase, decoration.trailing)
}

// TurnProgressSink posts and edits one progress line for a turn.
type TurnProgressSink interface {
	Post(ctx context.Context, notice string) (string, error)
	Edit(ctx context.Context, messageID, notice string) error
	Delete(ctx context.Context, messageID string) error
}

// worklogSink renders the richer surface. Every way it can be unavailable
// falls back to the notice lines. See docs/sirens-echo-worklog.md.
type worklogSink interface {
	WorklogAllowed(ctx context.Context) bool
	PostWorklog(ctx context.Context, view progressView) (string, error)
	EditWorklog(ctx context.Context, messageID string, view progressView) error
	// WorklogRefused reports an error that means the surface is unavailable
	// rather than that this one call failed.
	WorklogRefused(err error) bool
}

// turnProgress reports a long turn's stage. Every method is safe before the
// threshold, which is what keeps the fast path free of special cases.
type turnProgress struct {
	sink      TurnProgressSink
	telemetry *Telemetry
	start     time.Time
	now       func() time.Time

	mu        sync.Mutex
	postedAt  time.Time
	messageID string
	lastEdit  time.Time
	lastStage string
	finished  bool
	// carried marks a line repurposed as the turn's answer, so Finish leaves it.
	carried bool
	// waits is the elapsed seconds of each line appended while one stage runs.
	// Reset on a stage change, because the new line restarts the narration.
	waits []int
	// rows is the worklog, one entry per tool call, kept whole so the cap is a
	// rendering decision rather than a lossy one.
	rows []progressRow
	// stopped marks a turn that ended on something other than an answer, so the
	// element resolves rather than being deleted mid-narration.
	stopped bool
	// degraded latches the fallback once the richer surface is refused, so one
	// refusal does not become a refusal per edit.
	degraded bool
}

// progressWaitIcons are the twelve clock faces in order, so the hour hand
// advances as the lines pile up. Kai asked for the rotation on sirens-echo#370.
var progressWaitIcons = []string{
	"\U0001F550", "\U0001F551", "\U0001F552", "\U0001F553",
	"\U0001F554", "\U0001F555", "\U0001F556", "\U0001F557",
	"\U0001F558", "\U0001F559", "\U0001F55A", "\U0001F55B",
}

// progressWaitIcon is the face for the nth elapsed line. It wraps, so the line
// cap and the length of the rotation stay independent of each other.
func progressWaitIcon(line int) string {
	return progressWaitIcons[line%len(progressWaitIcons)]
}

// progressBody renders the stage line plus one line per beat waited, which is
// the shape Kai drew on sirens-echo#370.
func progressBody(phrase string, waits []int) string {
	body := stageLine(phrase)
	for index, seconds := range waits {
		// The clock line is the other one Kai terminated, whatever stage it
		// happens to be narrating. See sirens-echo#370.
		body += "\n" + stageNotice(
			progressWaitIcon(index),
			fmt.Sprintf("still %s %d seconds", phrase, seconds),
			true,
		)
	}
	return body
}

// richSink is the worklog surface when this turn may use it. The permission
// read happens at most once, because a refusal latches.
func (p *turnProgress) richSink(ctx context.Context) worklogSink {
	p.mu.Lock()
	degraded := p.degraded
	p.mu.Unlock()
	if degraded {
		return nil
	}
	sink, ok := p.sink.(worklogSink)
	if !ok || !sink.WorklogAllowed(ctx) {
		p.degrade()
		return nil
	}
	return sink
}

// degrade drops to the notice lines for the rest of the turn. Never fails the
// turn: a missing permission is a presentation fact. See sirens-echo#111.
func (p *turnProgress) degrade() {
	p.mu.Lock()
	p.degraded = true
	p.mu.Unlock()
}

// view is the worklog as the sink renders it. Called with the lock held.
func (p *turnProgress) view() progressView {
	title := worklogTitle
	if p.stopped {
		title = worklogStopped
	}
	return progressView{
		Title:    title,
		Rows:     worklogRows(p.rows),
		Elapsed:  p.now().Sub(p.start),
		Tools:    len(p.rows),
		Resolved: p.stopped,
	}
}

// postProgress posts on whichever surface this channel allows, falling back
// rather than failing when the richer one is refused.
func (p *turnProgress) postProgress(
	ctx context.Context, view progressView, notice string,
) (string, error) {
	if sink := p.richSink(ctx); sink != nil {
		posted, err := sink.PostWorklog(ctx, view)
		if err == nil {
			return posted, nil
		}
		if !sink.WorklogRefused(err) {
			return "", err
		}
		p.degrade()
	}
	return p.sink.Post(ctx, notice)
}

// editProgress is postProgress for a message that already exists.
func (p *turnProgress) editProgress(
	ctx context.Context, messageID string, view progressView, notice string,
) error {
	if sink := p.richSink(ctx); sink != nil {
		err := sink.EditWorklog(ctx, messageID, view)
		if err == nil || !sink.WorklogRefused(err) {
			return err
		}
		p.degrade()
	}
	return p.sink.Edit(ctx, messageID, notice)
}

// ToolStarted opens a worklog row. The row resolves in place rather than being
// replaced, which is what lets a member see progress rather than motion.
func (p *turnProgress) ToolStarted(server, tool string) {
	if p == nil || p.sink == nil {
		return
	}
	p.mu.Lock()
	defer p.mu.Unlock()
	if p.finished {
		return
	}
	p.rows = append(p.rows, progressRow{server: server, tool: tool})
}

// ToolFinished resolves the most recent unresolved row for that tool.
func (p *turnProgress) ToolFinished(server, tool string, outcome ToolOutcome) {
	if p == nil || p.sink == nil {
		return
	}
	p.mu.Lock()
	defer p.mu.Unlock()
	for index := len(p.rows) - 1; index >= 0; index-- {
		row := &p.rows[index]
		if row.done || row.server != server || row.tool != tool {
			continue
		}
		row.outcome = outcome
		row.done = true
		return
	}
}

// Stop marks a turn ending on something other than an answer, so Finish
// resolves the element rather than deleting it. See docs/sirens-echo-worklog.md.
func (p *turnProgress) Stop() {
	if p == nil || p.sink == nil {
		return
	}
	p.mu.Lock()
	p.stopped = true
	p.mu.Unlock()
}

// stopFromContext resolves the element from a layer holding no reference, the
// same route the stage narration takes.
func stopFromContext(ctx context.Context) {
	progress, _ := ctx.Value(turnProgressKey{}).(*turnProgress)
	progress.Stop()
}

func newTurnProgress(sink TurnProgressSink, now func() time.Time) *turnProgress {
	return newReportingTurnProgress(sink, nil, now)
}

// newReportingTurnProgress carries the telemetry handle. Without one the line is
// still posted, so a hand-built progress in a test needs no telemetry.
func newReportingTurnProgress(
	sink TurnProgressSink,
	telemetry *Telemetry,
	now func() time.Time,
) *turnProgress {
	if now == nil {
		now = time.Now
	}
	return &turnProgress{sink: sink, telemetry: telemetry, start: now(), now: now}
}

// record reports what the progress line did. Without it a rejected post and a
// turn too short to narrate look identical from outside.
func (p *turnProgress) record(ctx context.Context, action string, err error) {
	if p.telemetry == nil {
		return
	}
	if err != nil {
		p.telemetry.Info(
			ctx,
			"discord.progress.failed",
			slog.String("action", action),
			slog.String("error", err.Error()),
		)
		return
	}
	p.telemetry.Info(ctx, "discord.progress.posted", slog.String("action", action))
}

// Stage records what the turn is doing. It posts once the turn has run long
// enough to be worth narrating, and edits after that.
func (p *turnProgress) Stage(ctx context.Context, phrase string) {
	if p == nil || p.sink == nil {
		return
	}
	p.mu.Lock()
	if p.finished || phrase == p.lastStage {
		p.mu.Unlock()
		return
	}
	moment := p.now()
	if moment.Sub(p.start) < turnProgressAfter {
		p.lastStage = phrase
		p.mu.Unlock()
		return
	}
	if p.messageID != "" && moment.Sub(p.lastEdit) < turnProgressEvery {
		p.mu.Unlock()
		return
	}
	existing := p.messageID
	p.lastStage = phrase
	p.lastEdit = moment
	// A new stage restarts the narration, so the previous stage's waits go.
	p.waits = nil
	view := p.view()
	p.mu.Unlock()

	notice := progressBody(phrase, nil)
	if existing == "" {
		posted, err := p.postProgress(ctx, view, notice)
		p.record(ctx, "post", err)
		if err != nil {
			return
		}
		p.mu.Lock()
		// A reply may have landed while the post was in flight, in which case
		// the line is already unwanted.
		finished := p.finished
		p.messageID = posted
		p.postedAt = p.now()
		p.mu.Unlock()
		if finished {
			p.record(ctx, "delete", p.sink.Delete(ctx, posted))
		}
		return
	}
	p.record(ctx, "edit", p.editProgress(ctx, existing, view, notice))
}

// Watch narrates a turn that is waiting rather than changing stage. Stage alone
// only posts on a transition, and a long model call makes none.
func (p *turnProgress) Watch(ctx context.Context) func() {
	if p == nil || p.sink == nil {
		return func() {}
	}
	done := make(chan struct{})
	go func() {
		ticker := time.NewTicker(turnProgressEvery)
		defer ticker.Stop()
		for {
			select {
			case <-done:
				return
			case <-ctx.Done():
				return
			case <-ticker.C:
				p.refresh(ctx)
			}
		}
	}()
	return func() { close(done) }
}

// refresh posts the current stage once the turn has run long enough. A line
// already up says the right thing until Stage edits it.
func (p *turnProgress) refresh(ctx context.Context) {
	p.mu.Lock()
	if p.finished || p.lastStage == "" {
		p.mu.Unlock()
		return
	}
	// A line is already up, so the turn is waiting rather than starting. Say
	// how long instead of returning, which left it static. See #370.
	if p.messageID != "" {
		p.narrateWait(ctx)
		return
	}
	moment := p.now()
	if moment.Sub(p.start) < turnProgressAfter {
		p.mu.Unlock()
		return
	}
	phrase := p.lastStage
	p.lastEdit = moment
	view := p.view()
	p.mu.Unlock()

	posted, err := p.postProgress(ctx, view, stageLine(phrase))
	p.record(ctx, "post", err)
	if err != nil {
		return
	}
	p.mu.Lock()
	// A reply may have landed while the post was in flight, in which case the
	// line is already unwanted.
	finished := p.finished
	p.messageID = posted
	p.postedAt = p.now()
	p.mu.Unlock()
	if finished {
		_ = p.sink.Delete(ctx, posted)
	}
}

// longEnough reports that the turn has run past the window where its reply
// wants somewhere of its own. See docs/sirens-echo-threads.md.
func (p *turnProgress) longEnough() bool {
	if p == nil || p.sink == nil {
		return false
	}
	p.mu.Lock()
	defer p.mu.Unlock()
	// The posted line is the channel-side announcement, so a turn without one
	// has nothing pointing at a thread and does not get one.
	if p.postedAt.IsZero() {
		return false
	}
	return p.now().Sub(p.start) >= turnLongReplyAfter
}

// turnLongReply reports whether this turn's reply belongs in a thread.
func turnLongReply(ctx context.Context) bool {
	progress, _ := ctx.Value(turnProgressKey{}).(*turnProgress)
	return progress.longEnough()
}

// turnProgressKey carries the turn's progress line to the tool loop, which
// lives behind the completion boundary and takes no progress argument.
type turnProgressKey struct{}

// WithTurnProgress marks a context as narrating one turn.
func WithTurnProgress(ctx context.Context, progress *turnProgress) context.Context {
	if progress == nil {
		return ctx
	}
	return context.WithValue(ctx, turnProgressKey{}, progress)
}

// reportStage narrates from a layer that holds no progress reference. A context
// without one is the ordinary case for a non-Discord transport.
func reportStage(ctx context.Context, phrase string) {
	progress, _ := ctx.Value(turnProgressKey{}).(*turnProgress)
	progress.Stage(ctx, phrase)
}

// reportToolStarted and reportToolFinished open and resolve one worklog row
// from the completion layer, which holds no progress reference.
func reportToolStarted(ctx context.Context, server, tool string) {
	progress, _ := ctx.Value(turnProgressKey{}).(*turnProgress)
	progress.ToolStarted(server, tool)
}

func reportToolFinished(ctx context.Context, server, tool string, outcome ToolOutcome) {
	progress, _ := ctx.Value(turnProgressKey{}).(*turnProgress)
	progress.ToolFinished(server, tool, outcome)
}

// settleDelay is how long until the next beat of the grid the line started. A
// turn that never posted one returns zero, so an ordinary reply is never held.
func (p *turnProgress) settleDelay() time.Duration {
	if p == nil || p.sink == nil {
		return 0
	}
	p.mu.Lock()
	posted := p.postedAt
	p.mu.Unlock()
	if posted.IsZero() {
		return 0
	}
	elapsed := p.now().Sub(posted)
	if elapsed <= 0 {
		return 0
	}
	// Landing exactly on a beat is already on time, so only a remainder waits.
	if remainder := elapsed % turnProgressEvery; remainder > 0 {
		return turnProgressEvery - remainder
	}
	return 0
}

// Settle waits out that delay, or returns early when the turn is cancelled.
func (p *turnProgress) Settle(ctx context.Context) {
	remaining := p.settleDelay()
	if remaining <= 0 {
		return
	}
	timer := time.NewTimer(remaining)
	defer timer.Stop()
	select {
	case <-ctx.Done():
	case <-timer.C:
	}
}

// settleFromContext holds a reply from a layer that carries no progress
// reference, which is how the failure path reaches it.
func settleFromContext(ctx context.Context) {
	progress, _ := ctx.Value(turnProgressKey{}).(*turnProgress)
	progress.Settle(ctx)
}

// settleDelayFromContext reports the hold a caller is about to take, so the
// span can carry it before the wait rather than after. See sirens-echo#652.
func settleDelayFromContext(ctx context.Context) time.Duration {
	progress, _ := ctx.Value(turnProgressKey{}).(*turnProgress)
	return progress.settleDelay()
}

// Carry turns the narration into the turn's answer, for a notice that could not
// be sent. See docs/sirens-echo-delivery.md.
func (p *turnProgress) Carry(ctx context.Context, notice string) {
	if p == nil || p.sink == nil || strings.TrimSpace(notice) == "" {
		return
	}
	p.mu.Lock()
	messageID := p.messageID
	if p.finished || messageID == "" {
		p.mu.Unlock()
		return
	}
	// Claimed before the edit is attempted, so a line that could not be updated
	// is left rather than deleted. A stale stage beats nothing at all.
	p.carried = true
	p.mu.Unlock()
	// An edit is a different call from a send, against a message that already
	// exists, so it can land where the send did not.
	p.record(ctx, "carry", p.sink.Edit(ctx, messageID, notice))
}

// carryFromContext repurposes the line from a layer that holds no progress
// reference, the same route the stage narration already takes.
func carryFromContext(ctx context.Context, notice string) {
	progress, _ := ctx.Value(turnProgressKey{}).(*turnProgress)
	progress.Carry(ctx, notice)
}

// Finish removes the progress line. The reply or the failure notice is the
// turn's real answer, so the narration does not outlive it.
func (p *turnProgress) Finish(ctx context.Context) {
	if p == nil || p.sink == nil {
		return
	}
	p.mu.Lock()
	if p.finished {
		p.mu.Unlock()
		return
	}
	p.finished = true
	messageID := p.messageID
	carried := p.carried
	stopped := p.stopped
	view := p.view()
	p.messageID = ""
	p.mu.Unlock()
	// A line carrying the notice is the answer. Deleting it here would remove
	// the only thing the member got. See sirens-echo#624.
	if carried {
		return
	}
	if messageID == "" {
		return
	}
	// A turn that stopped resolves the element rather than deleting it. An
	// element that merely vanishes is the #137 silence again. See #111.
	if stopped {
		p.record(ctx, "resolve", p.editProgress(
			ctx, messageID, view, harnessNotice(worklogStopped)))
		return
	}
	// A delivered answer is its own resolution, and the disclosure footer under
	// it already names the tools. Two lists of them is the thing #385 avoided.
	p.record(ctx, "delete", p.sink.Delete(ctx, messageID))
}

// narrateWait appends one elapsed line to the posted stage line. Called with
// the lock held, and it releases before touching the sink.
func (p *turnProgress) narrateWait(ctx context.Context) {
	if p.carried || len(p.waits) >= maxProgressWaitLines {
		p.mu.Unlock()
		return
	}
	moment := p.now()
	elapsed := moment.Sub(p.postedAt)
	if elapsed < turnProgressEvery {
		p.mu.Unlock()
		return
	}
	p.waits = append(p.waits, int(elapsed.Round(time.Second).Seconds()))
	p.lastEdit = moment
	existing, phrase := p.messageID, p.lastStage
	waits := append([]int{}, p.waits...)
	view := p.view()
	p.mu.Unlock()

	p.record(ctx, "edit", p.editProgress(ctx, existing, view, progressBody(phrase, waits)))
}
