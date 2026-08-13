package community

import (
	"context"
	"errors"
	"strings"
	"sync"
	"testing"
	"time"
)

// The five acceptance rows of sirens-echo#356, one test each where they are
// separable. Progress is a status line; content is an answer.

// recordingContentSink keeps what it was handed, in order, so a row can assert
// arrival and sequence rather than that no error came back.
type recordingContentSink struct {
	mu   sync.Mutex
	sent []string
	err  error
}

func (s *recordingContentSink) EmitJobContent(_ context.Context, _ Job, content string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.err != nil {
		return s.err
	}
	s.sent = append(s.sent, content)
	return nil
}

func (s *recordingContentSink) messages() []string {
	s.mu.Lock()
	defer s.mu.Unlock()
	return append([]string{}, s.sent...)
}

// contentRunner is a started-enough runner: contentFor reads only the counter
// and the two fields, so a full Start would add nothing this asserts.
func contentRunner(sink JobContentReporter, validate func(string) error) *JobRunner {
	return &JobRunner{
		Telemetry:       telemetryOrNoop(nil),
		Content:         sink,
		ValidateContent: validate,
		content:         newContentCounter(nil),
	}
}

func notifiableJob() Job {
	return Job{
		ID:     "job-1",
		Kind:   "test",
		Origin: JobOrigin{Transport: transportDiscord, ChannelID: "1024000000000000001"},
	}
}

func allow(string) error { return nil }

// Row 1: an ordered sequence arrives, and every one of it arrives.
func TestEveryContentMessageArrivesInOrder(t *testing.T) {
	t.Parallel()
	sink := &recordingContentSink{}
	emit := contentRunner(sink, allow).contentFor(context.Background(), notifiableJob())

	want := []string{"first part.", "second part.", "third part."}
	for _, part := range want {
		if err := emit(part); err != nil {
			t.Fatalf("emit %q: %v", part, err)
		}
	}
	got := sink.messages()
	if len(got) != len(want) {
		t.Fatalf("sent %d messages, want %d: %v", len(got), len(want), got)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Errorf("message %d = %q, want %q", i, got[i], want[i])
		}
	}
}

// Row 2: what a reply refuses, content refuses. Asserted through the runner
// rather than the validator, so a seam that forgets to call it fails here.
func TestContentIsRefusedForTheSameReasonAReplyIs(t *testing.T) {
	t.Parallel()
	sink := &recordingContentSink{}
	refuse := func(content string) error {
		if strings.Contains(content, "<|tool_call|>") {
			return errors.New("model reply carried tool-call markup")
		}
		return nil
	}
	emit := contentRunner(sink, refuse).contentFor(context.Background(), notifiableJob())

	if err := emit("Here it is <|tool_call|> for you."); err == nil {
		t.Fatal("content carrying tool-call markup was emitted")
	}
	if sent := sink.messages(); len(sent) != 0 {
		t.Errorf("a refused message still reached the origin: %v", sent)
	}
	// The path still works for content that passes, or the row above proves
	// only that everything is refused.
	if err := emit("Here it is."); err != nil {
		t.Fatalf("a clean message was refused: %v", err)
	}
}

// Row 2, the other direction: an unwired validator must not become an
// unchecked path to a member. See sirens-echo#621 for that shape.
func TestContentWithoutAValidatorIsRefused(t *testing.T) {
	t.Parallel()
	sink := &recordingContentSink{}
	emit := contentRunner(sink, nil).contentFor(context.Background(), notifiableJob())

	if err := emit("anything at all"); err == nil {
		t.Fatal("content was emitted with no validator configured")
	}
	if sent := sink.messages(); len(sent) != 0 {
		t.Errorf("unvalidated content reached the origin: %v", sent)
	}
}

// Row 5: the bound is stated and it refuses rather than dropping, because a
// missing paragraph is a hole in an answer and a dropped status line is not.
func TestTheFloodBoundRefusesRatherThanDropping(t *testing.T) {
	t.Parallel()
	sink := &recordingContentSink{}
	emit := contentRunner(sink, allow).contentFor(context.Background(), notifiableJob())

	for i := 0; i < maxJobContentMessages; i++ {
		if err := emit("part"); err != nil {
			t.Fatalf("message %d of the allowance was refused: %v", i+1, err)
		}
	}
	err := emit("one too many")
	if !errors.Is(err, ErrJobContentExhausted) {
		t.Fatalf("past the bound the error was %v, want ErrJobContentExhausted", err)
	}
	if sent := sink.messages(); len(sent) != maxJobContentMessages {
		t.Errorf("sent %d messages, want the bound %d", len(sent), maxJobContentMessages)
	}
}

// The window opens on the first message and closes ten minutes later, which
// is the other half of the ceiling Kai decided on sirens-echo#236.
func TestTheAnswerWindowClosesOnTime(t *testing.T) {
	t.Parallel()
	moment := time.Now()
	counter := &contentCounter{
		sent:    make(map[string]int),
		started: make(map[string]time.Time),
		now:     func() time.Time { return moment },
	}
	if err := counter.admit("job-1"); err != nil {
		t.Fatalf("the first message was refused: %v", err)
	}
	// Inside the window, and well under the message bound.
	moment = moment.Add(maxJobContentWindow - time.Second)
	if err := counter.admit("job-1"); err != nil {
		t.Fatalf("a message inside the window was refused: %v", err)
	}
	moment = moment.Add(2 * time.Second)
	if err := counter.admit("job-1"); !errors.Is(err, ErrJobContentWindowClosed) {
		t.Fatalf("past the window the error was %v, want ErrJobContentWindowClosed", err)
	}
}

// Queue time is not answer time, so the window opens on the first message
// rather than when the job was submitted.
func TestTheWindowOpensOnTheFirstMessage(t *testing.T) {
	t.Parallel()
	moment := time.Now()
	counter := &contentCounter{
		sent:    make(map[string]int),
		started: make(map[string]time.Time),
		now:     func() time.Time { return moment },
	}
	// A job that sat in the queue for an hour still gets its full window.
	moment = moment.Add(time.Hour)
	if err := counter.admit("job-1"); err != nil {
		t.Fatalf("the first message after a long queue was refused: %v", err)
	}
	moment = moment.Add(maxJobContentWindow / 2)
	if err := counter.admit("job-1"); err != nil {
		t.Errorf("a message half a window later was refused: %v", err)
	}
}

// The two ceilings are distinguishable, or an operator cannot tell a job that
// said too much from one that took too long.
func TestTheTwoCeilingsAreDistinguishable(t *testing.T) {
	t.Parallel()
	if errors.Is(ErrJobContentExhausted, ErrJobContentWindowClosed) {
		t.Error("the message bound and the time bound are the same error")
	}
	if !strings.Contains(ErrJobContentWindowClosed.Error(), "window") {
		t.Errorf("the time ceiling does not name itself: %v", ErrJobContentWindowClosed)
	}
}

// The bound is per job, or one long job silences the next one.
func TestTheBoundIsPerJob(t *testing.T) {
	t.Parallel()
	sink := &recordingContentSink{}
	runner := contentRunner(sink, allow)
	first := runner.contentFor(context.Background(), notifiableJob())
	for i := 0; i < maxJobContentMessages; i++ {
		if err := first("part"); err != nil {
			t.Fatalf("filling the first job: %v", err)
		}
	}
	other := notifiableJob()
	other.ID = "job-2"
	if err := runner.contentFor(context.Background(), other)("first for the second job"); err != nil {
		t.Errorf("a second job inherited the first job's exhaustion: %v", err)
	}
}

// Row 4: a job that emits nothing behaves exactly as it does today, so an
// executor that never calls this touches none of it.
func TestAJobThatEmitsNothingTouchesNothing(t *testing.T) {
	t.Parallel()
	sink := &recordingContentSink{}
	runner := contentRunner(sink, allow)
	_ = runner.contentFor(context.Background(), notifiableJob())
	if sent := sink.messages(); len(sent) != 0 {
		t.Errorf("building the emitter sent something: %v", sent)
	}
}

// An origin that cannot receive is refused rather than counted, so a job on a
// transport with nowhere to answer does not silently spend its allowance.
func TestAnUnnotifiableOriginIsRefused(t *testing.T) {
	t.Parallel()
	sink := &recordingContentSink{}
	quiet := notifiableJob()
	quiet.Origin = JobOrigin{Transport: transportHTTP}
	if err := contentRunner(sink, allow).contentFor(context.Background(), quiet)("x"); err == nil {
		t.Fatal("content was emitted to an origin that cannot receive it")
	}
}

// Empty content is a caller bug rather than a message. Sending it would leave
// a blank line in the middle of an answer.
func TestEmptyContentIsRefused(t *testing.T) {
	t.Parallel()
	sink := &recordingContentSink{}
	emit := contentRunner(sink, allow).contentFor(context.Background(), notifiableJob())
	for _, blank := range []string{"", "   ", "\n\t"} {
		if err := emit(blank); err == nil {
			t.Errorf("blank content %q was emitted", blank)
		}
	}
}

// Row 3, stated where it can be: content and progress do not share the
// counter, so emitting never consumes a progress slot or the reverse.
func TestContentAndProgressDoNotShareABound(t *testing.T) {
	t.Parallel()
	counter := newContentCounter(nil)
	limiter := newProgressLimiter(nil)
	for i := 0; i < maxJobContentMessages; i++ {
		if err := counter.admit("job-1"); err != nil {
			t.Fatalf("content allowance ended at %d: %v", i, err)
		}
	}
	if !limiter.admit("job-1") {
		t.Error("a job that spent its content allowance lost its progress line")
	}
}

// A re-execution repeats the executor's emits. The member must not read the
// same paragraph twice. See sirens-echo#621.

// storedJob puts a real job in a real store, so the effect guard is exercised
// against the machinery rather than a stub of it.
func storedJob(t *testing.T) (*MemoryJobStore, Job) {
	t.Helper()
	store := NewMemoryJobStore(nil)
	job, _, err := store.Submit(Job{
		ID:             "job-content-replay",
		Kind:           "test",
		Principal:      "318190481467244544",
		IdempotencyKey: "job-content-replay-key",
		State:          JobRunning,
		Origin:         JobOrigin{Transport: transportDiscord, ChannelID: "1024000000000000001"},
	})
	if err != nil {
		t.Fatalf("submit: %v", err)
	}
	return store, job
}

func TestAReplayedMessageIsNotSentTwice(t *testing.T) {
	t.Parallel()
	store, job := storedJob(t)
	sink := &recordingContentSink{}
	runner := &JobRunner{
		Telemetry: telemetryOrNoop(nil), Store: store,
		Content: sink, ValidateContent: allow, content: newContentCounter(nil),
	}
	first := runner.contentFor(context.Background(), job)
	for _, part := range []string{"one.", "two."} {
		if err := first(part); err != nil {
			t.Fatalf("first run: %v", err)
		}
	}

	// A re-execution: same job, same store, a fresh counter, same emits.
	runner.content = newContentCounter(nil)
	replay := runner.contentFor(context.Background(), job)
	for _, part := range []string{"one.", "two."} {
		if err := replay(part); err != nil {
			t.Fatalf("replay: %v", err)
		}
	}
	if sent := sink.messages(); len(sent) != 2 {
		t.Errorf("the member received %d messages across a replay, want 2: %v",
			len(sent), sent)
	}
}

// The replay skips only what was delivered. A message the first run never
// reached still arrives, or a crash mid-answer truncates it forever.
func TestAReplayStillDeliversWhatTheFirstRunDidNot(t *testing.T) {
	t.Parallel()
	store, job := storedJob(t)
	sink := &recordingContentSink{}
	runner := &JobRunner{
		Telemetry: telemetryOrNoop(nil), Store: store,
		Content: sink, ValidateContent: allow, content: newContentCounter(nil),
	}
	if err := runner.contentFor(context.Background(), job)("one."); err != nil {
		t.Fatalf("first run: %v", err)
	}

	runner.content = newContentCounter(nil)
	replay := runner.contentFor(context.Background(), job)
	if err := replay("one."); err != nil {
		t.Fatalf("replay of the delivered message: %v", err)
	}
	if err := replay("two."); err != nil {
		t.Fatalf("the undelivered message was refused: %v", err)
	}
	sent := sink.messages()
	if len(sent) != 2 || sent[0] != "one." || sent[1] != "two." {
		t.Errorf("member received %v, want [one. two.]", sent)
	}
}

// The effect is recorded after the send. Recording first would skip a message
// the origin never received, which is worse than sending it twice.
func TestAFailedSendRecordsNoEffect(t *testing.T) {
	t.Parallel()
	store, job := storedJob(t)
	sink := &recordingContentSink{err: errors.New("discord refused")}
	runner := &JobRunner{
		Telemetry: telemetryOrNoop(nil), Store: store,
		Content: sink, ValidateContent: allow, content: newContentCounter(nil),
	}
	if err := runner.contentFor(context.Background(), job)("one."); err == nil {
		t.Fatal("a failed send reported success")
	}
	current, err := store.Get(job.ID)
	if err != nil {
		t.Fatalf("get: %v", err)
	}
	if EffectApplied(current, contentEffectStep(1)) {
		t.Error("a message the origin never received was recorded as delivered")
	}
}

// Without a store the path still works, because the in-memory deployment has
// nothing to resume from and the guard must not become a requirement.
func TestContentWorksWithoutAStore(t *testing.T) {
	t.Parallel()
	sink := &recordingContentSink{}
	runner := contentRunner(sink, allow)
	if err := runner.contentFor(context.Background(), notifiableJob())("one."); err != nil {
		t.Fatalf("emitting without a store: %v", err)
	}
	if len(sink.messages()) != 1 {
		t.Error("the message did not reach the origin")
	}
}
