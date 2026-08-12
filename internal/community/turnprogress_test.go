package community

import (
	"context"
	"errors"
	"sync"
	"testing"
	"time"
)

type recordingSink struct {
	mu      sync.Mutex
	posts   []string
	edits   []string
	deletes []string
	postErr error
}

func (s *recordingSink) Post(_ context.Context, notice string) (string, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.postErr != nil {
		return "", s.postErr
	}
	s.posts = append(s.posts, notice)
	return "message-1", nil
}

func (s *recordingSink) Edit(_ context.Context, _, notice string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.edits = append(s.edits, notice)
	return nil
}

func (s *recordingSink) Delete(_ context.Context, messageID string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.deletes = append(s.deletes, messageID)
	return nil
}

func (s *recordingSink) counts() (int, int, int) {
	s.mu.Lock()
	defer s.mu.Unlock()
	return len(s.posts), len(s.edits), len(s.deletes)
}

// stepClock advances only when a test says so, so nothing sleeps.
func stepClock(start time.Time) (func() time.Time, func(time.Duration)) {
	var mu sync.Mutex
	current := start
	now := func() time.Time {
		mu.Lock()
		defer mu.Unlock()
		return current
	}
	advance := func(by time.Duration) {
		mu.Lock()
		current = current.Add(by)
		mu.Unlock()
	}
	return now, advance
}

// A reply that beats the threshold posts nothing, so an ordinary turn is
// untouched. See #111.
func TestAShortTurnPostsNoProgress(t *testing.T) {
	t.Parallel()
	sink := &recordingSink{}
	now, advance := stepClock(time.Unix(1700000000, 0).UTC())
	progress := newTurnProgress(sink, now)

	progress.Stage(context.Background(), stagePhraseHistory)
	advance(time.Second)
	progress.Stage(context.Background(), stagePhraseThinking)
	progress.Finish(context.Background())

	posts, edits, deletes := sink.counts()
	if posts != 0 || edits != 0 || deletes != 0 {
		t.Errorf("a short turn touched Discord: %d posts, %d edits, %d deletes",
			posts, edits, deletes)
	}
}

// A long turn posts one line and edits it, rather than posting a column.
func TestALongTurnEditsOneLineAndRemovesIt(t *testing.T) {
	t.Parallel()
	sink := &recordingSink{}
	now, advance := stepClock(time.Unix(1700000000, 0).UTC())
	progress := newTurnProgress(sink, now)

	progress.Stage(context.Background(), stagePhraseHistory)
	advance(turnProgressAfter + time.Second)
	progress.Stage(context.Background(), stagePhraseThinking)
	advance(turnProgressEvery + time.Second)
	progress.Stage(context.Background(), stagePhraseTool)
	advance(turnProgressEvery + time.Second)
	progress.Stage(context.Background(), stagePhraseChecking)

	posts, edits, _ := sink.counts()
	if posts != 1 {
		t.Errorf("posted %d lines, want 1", posts)
	}
	if edits != 2 {
		t.Errorf("edited %d times, want 2", edits)
	}
	// The reply is the turn's answer, so the narration does not outlive it.
	progress.Finish(context.Background())
	if _, _, deletes := sink.counts(); deletes != 1 {
		t.Errorf("deleted %d lines, want 1", deletes)
	}
}

// Edits are bounded, so a tool-heavy turn cannot spend its budget on Discord.
func TestProgressEditsAreRateLimited(t *testing.T) {
	t.Parallel()
	sink := &recordingSink{}
	now, advance := stepClock(time.Unix(1700000000, 0).UTC())
	progress := newTurnProgress(sink, now)

	advance(turnProgressAfter + time.Second)
	progress.Stage(context.Background(), stagePhraseThinking)
	for index := 0; index < 5; index++ {
		advance(time.Millisecond)
		progress.Stage(context.Background(), stagePhraseTool)
		progress.Stage(context.Background(), stagePhraseThinking)
	}
	posts, edits, _ := sink.counts()
	if posts != 1 {
		t.Errorf("posted %d lines", posts)
	}
	if edits != 0 {
		t.Errorf("made %d edits inside the window, want 0", edits)
	}
}

// The line must not outlive the turn even when the reply wins the race.
func TestAFinishedTurnRemovesALineThatArrivedLate(t *testing.T) {
	t.Parallel()
	sink := &recordingSink{}
	now, advance := stepClock(time.Unix(1700000000, 0).UTC())
	progress := newTurnProgress(sink, now)
	advance(turnProgressAfter + time.Second)

	progress.mu.Lock()
	progress.finished = true
	progress.mu.Unlock()
	// Stage refuses once finished, which is the ordinary case.
	progress.Stage(context.Background(), stagePhraseThinking)
	if posts, _, _ := sink.counts(); posts != 0 {
		t.Errorf("posted after the turn finished: %d", posts)
	}
}

// A failed post must not break the turn, because progress is advisory.
func TestAFailedProgressPostIsNotFatal(t *testing.T) {
	t.Parallel()
	sink := &recordingSink{postErr: errors.New("discord said no")}
	now, advance := stepClock(time.Unix(1700000000, 0).UTC())
	progress := newTurnProgress(sink, now)
	advance(turnProgressAfter + time.Second)

	progress.Stage(context.Background(), stagePhraseThinking)
	progress.Finish(context.Background())
	if _, _, deletes := sink.counts(); deletes != 0 {
		t.Errorf("deleted a line that was never posted: %d", deletes)
	}
}

// A nil progress is the non-Discord path and every method has to accept it.
func TestNilProgressIsSafe(t *testing.T) {
	t.Parallel()
	var progress *turnProgress
	progress.Stage(context.Background(), stagePhraseThinking)
	progress.Finish(context.Background())
}

func TestProgressStagesUseTheHarnessFormat(t *testing.T) {
	t.Parallel()
	for _, phrase := range []string{
		stagePhraseHistory, stagePhraseThinking, stagePhraseTool, stagePhraseChecking,
	} {
		if notice := harnessNotice(phrase); !noticeShape.MatchString(notice) {
			t.Errorf("stage %q renders %q, which is not the harness shape", phrase, notice)
		}
	}
}
