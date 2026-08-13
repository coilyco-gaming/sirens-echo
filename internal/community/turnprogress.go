package community

import (
	"context"
	"sync"
	"time"
)

// A long turn says what it is doing. A short one is untouched, because a
// progress line for a two-second reply is noise. See docs/sirens-echo-progress.md.

const (
	// turnProgressAfter is how long a turn runs before it starts reporting. A
	// reply that beats this never posts anything.
	turnProgressAfter = 8 * time.Second
	// turnProgressEvery bounds edits, so a tool-heavy turn cannot spend its
	// budget on Discord calls.
	turnProgressEvery = 4 * time.Second
)

// The stage phrases. They come from the closed notice vocabulary, so a progress
// line reads as the harness speaking rather than as the model.
const (
	stagePhraseHistory  = "reading recent messages"
	stagePhraseThinking = "thinking"
	stagePhraseTool     = "calling a tool"
	stagePhraseChecking = "checking the reply"
)

// TurnProgressSink posts and edits one progress line for a turn.
type TurnProgressSink interface {
	Post(ctx context.Context, notice string) (string, error)
	Edit(ctx context.Context, messageID, notice string) error
	Delete(ctx context.Context, messageID string) error
}

// turnProgress reports a long turn's stage. Every method is safe before the
// threshold, which is what keeps the fast path free of special cases.
type turnProgress struct {
	sink  TurnProgressSink
	start time.Time
	now   func() time.Time

	mu        sync.Mutex
	messageID string
	lastEdit  time.Time
	lastStage string
	finished  bool
}

func newTurnProgress(sink TurnProgressSink, now func() time.Time) *turnProgress {
	if now == nil {
		now = time.Now
	}
	return &turnProgress{sink: sink, start: now(), now: now}
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
	p.mu.Unlock()

	notice := harnessNotice(phrase)
	if existing == "" {
		posted, err := p.sink.Post(ctx, notice)
		if err != nil {
			return
		}
		p.mu.Lock()
		// A reply may have landed while the post was in flight, in which case
		// the line is already unwanted.
		finished := p.finished
		p.messageID = posted
		p.mu.Unlock()
		if finished {
			_ = p.sink.Delete(ctx, posted)
		}
		return
	}
	_ = p.sink.Edit(ctx, existing, notice)
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
	if p.finished || p.lastStage == "" || p.messageID != "" {
		p.mu.Unlock()
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

	posted, err := p.sink.Post(ctx, harnessNotice(phrase))
	if err != nil {
		return
	}
	p.mu.Lock()
	// A reply may have landed while the post was in flight, in which case the
	// line is already unwanted.
	finished := p.finished
	p.messageID = posted
	p.mu.Unlock()
	if finished {
		_ = p.sink.Delete(ctx, posted)
	}
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
	p.messageID = ""
	p.mu.Unlock()
	if messageID != "" {
		_ = p.sink.Delete(ctx, messageID)
	}
}
