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

// stageIcons decorate a progress line. Only thinking is specified so far, and
// an unlisted stage renders plain. See sirens-echo#370.
var stageIcons = map[string]string{
	stagePhraseThinking: "\U0001F914",
}

// TurnProgressSink posts and edits one progress line for a turn.
type TurnProgressSink interface {
	Post(ctx context.Context, notice string) (string, error)
	Edit(ctx context.Context, messageID, notice string) error
	Delete(ctx context.Context, messageID string) error
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
}

// progressWaitIcon leads a line that says only that time passed. One clock,
// not a rotation: Kai wrote "if we wanna" for that, which is not a spec.
const progressWaitIcon = "\U0001F550"

// maxProgressWaitLines bounds the column a stuck turn can grow. The turn
// ceiling over the beat is the natural count. See sirens-echo#370.
const maxProgressWaitLines = 12

// progressBody renders the stage line plus one line per beat waited, which is
// the shape Kai drew on sirens-echo#370.
func progressBody(phrase string, waits []int) string {
	body := stageNotice(stageIcons[phrase], phrase)
	for _, seconds := range waits {
		body += "\n" + stageNotice(
			progressWaitIcon, fmt.Sprintf("still %s %d seconds", phrase, seconds))
	}
	return body
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
	p.mu.Unlock()

	notice := progressBody(phrase, nil)
	if existing == "" {
		posted, err := p.sink.Post(ctx, notice)
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
	p.record(ctx, "edit", p.sink.Edit(ctx, existing, notice))
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
	p.mu.Unlock()

	posted, err := p.sink.Post(ctx, stageNotice(stageIcons[phrase], phrase))
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
// be sent. See docs/sirens-echo-delivery-failures.md.
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
	p.messageID = ""
	p.mu.Unlock()
	// A line carrying the notice is the answer. Deleting it here would remove
	// the only thing the member got. See sirens-echo#624.
	if carried {
		return
	}
	if messageID != "" {
		p.record(ctx, "delete", p.sink.Delete(ctx, messageID))
	}
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
	p.mu.Unlock()

	p.record(ctx, "edit", p.sink.Edit(ctx, existing, progressBody(phrase, waits)))
}
