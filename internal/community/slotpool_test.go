package community

import (
	"context"
	"errors"
	"testing"
	"time"
)

// blockingClient holds every turn inside the model call until the test lets it
// go, which is the only way to observe how many turns hold a slot at once.
type blockingClient struct {
	arrived chan struct{}
	release chan struct{}
	err     error
}

func (c blockingClient) Complete(
	ctx context.Context, _ TurnPrompt, _ string,
) (CompletionResult, error) {
	c.arrived <- struct{}{}
	select {
	case <-c.release:
	case <-ctx.Done():
		return CompletionResult{}, ctx.Err()
	}
	if c.err != nil {
		return CompletionResult{}, c.err
	}
	return CompletionResult{Content: "Echo is ready."}, nil
}

func pooledAgent(t *testing.T, client CompletionClient, slots int) *Agent {
	t.Helper()
	agent := &Agent{
		cfg: Config{
			Definition:     Definition{MaxContextMessages: 12},
			ExecutionSlots: slots,
			QueueTimeout:   5 * time.Second,
			RequestTimeout: 5 * time.Second,
		},
		completions:  client,
		systemPrompt: "neutral model policy and local knowledge",
		telemetry:    telemetryOrNoop(nil),
	}
	agent.ensureRuntimeDefaults()
	return agent
}

// awaitArrivals fails rather than hanging when fewer turns reach the model than
// the pool should allow, so a regression to one slot reports what it did.
func awaitArrivals(t *testing.T, client blockingClient, want int) {
	t.Helper()
	deadline := time.After(5 * time.Second)
	for held := range want {
		select {
		case <-client.arrived:
		case <-deadline:
			t.Fatalf("only %d of %d turns reached the model, so the pool still serialises", held, want)
		}
	}
}

// The whole point of the pool. A 20-way burst admitted eight and ran them one
// after another, so a second summon queued behind a three-minute turn. #995.
func TestEightSummonsRunAtOnceRatherThanInSequence(t *testing.T) {
	t.Parallel()
	client := blockingClient{arrived: make(chan struct{}, 16), release: make(chan struct{})}
	agent := pooledAgent(t, client, 8)

	for index := range 8 {
		go func() {
			_ = agent.runSerialized(context.Background(), &httpTurn{
				requestID: string(rune('a' + index)),
				current:   TranscriptEntry{Author: "member", Content: "are you ready?"},
			})
		}()
	}

	awaitArrivals(t, client, 8)
	close(client.release)
}

// A ninth caller waits rather than being shed. The single-slot queue timeout
// shed at 30s while the turn ahead of it had three minutes to run.
func TestANinthSummonWaitsForASlotRatherThanBeingShed(t *testing.T) {
	t.Parallel()
	client := blockingClient{arrived: make(chan struct{}, 16), release: make(chan struct{})}
	agent := pooledAgent(t, client, 8)

	for index := range 8 {
		go func() {
			_ = agent.runSerialized(context.Background(), &httpTurn{
				requestID: string(rune('a' + index)),
				current:   TranscriptEntry{Author: "member", Content: "are you ready?"},
			})
		}()
	}
	awaitArrivals(t, client, 8)

	ninth := make(chan error, 1)
	go func() {
		ninth <- agent.runSerialized(context.Background(), &httpTurn{
			requestID: "ninth",
			current:   TranscriptEntry{Author: "member", Content: "are you ready?"},
		})
	}()

	select {
	case err := <-ninth:
		t.Fatalf("the ninth turn finished while the pool was full: %v", err)
	case <-time.After(100 * time.Millisecond):
	}

	close(client.release)
	select {
	case <-client.arrived:
	case <-time.After(5 * time.Second):
		t.Fatal("the ninth turn never reached the model after a slot freed")
	}
	select {
	case err := <-ninth:
		if err != nil {
			t.Fatalf("the ninth turn failed: %v", err)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("the ninth turn never returned")
	}
}

// A slot the failure path keeps is a slot the pool never gets back, and eight
// of them stop the deployment answering entirely.
func TestAFailedTurnStillReturnsItsSlot(t *testing.T) {
	t.Parallel()
	release := make(chan struct{})
	close(release)
	client := blockingClient{
		arrived: make(chan struct{}, 64),
		release: release,
		err:     errors.New("the model refused"),
	}
	agent := pooledAgent(t, client, 2)

	for attempt := range 6 {
		_ = agent.runSerialized(context.Background(), &httpTurn{
			requestID: string(rune('a' + attempt)),
			current:   TranscriptEntry{Author: "member", Content: "are you ready?"},
		})
	}
	if held := len(agent.slots); held != 0 {
		t.Errorf("%d slots are still held after every turn finished", held)
	}
}

// A turn that gave up waiting never took a slot, so the pool must be untouched
// by its own shedding.
func TestASheddedTurnLeavesThePoolAlone(t *testing.T) {
	t.Parallel()
	client := blockingClient{arrived: make(chan struct{}, 16), release: make(chan struct{})}
	agent := pooledAgent(t, client, 1)
	agent.cfg.QueueTimeout = 20 * time.Millisecond

	go func() {
		_ = agent.runSerialized(context.Background(), &httpTurn{
			requestID: "holder",
			current:   TranscriptEntry{Author: "member", Content: "are you ready?"},
		})
	}()
	<-client.arrived

	err := agent.runSerialized(context.Background(), &httpTurn{
		requestID: "shed",
		current:   TranscriptEntry{Author: "member", Content: "are you ready?"},
	})
	if err == nil {
		t.Fatal("the second turn was admitted while the only slot was held")
	}
	if held := len(agent.slots); held != 1 {
		t.Errorf("the pool holds %d slots, want the one the running turn took", held)
	}
	close(client.release)
}

// The pool and the admission bound are one shape. MaxPending counts a turn from
// acceptance to release, so a bound at the pool size leaves no queue at all.
func TestTheAdmissionBoundLeavesAQueueBehindThePool(t *testing.T) {
	// Not parallel. applyKnobs writes the package defaults every other test
	// reads, which is why no knob test takes t.Parallel.
	restoreKnobs(t)
	applyKnobs(func(string) string { return "" })
	if defaultRateLimitPolicy.MaxPending <= executionSlots {
		t.Errorf(
			"MaxPending = %d with %d slots, so nothing may queue behind a full pool",
			defaultRateLimitPolicy.MaxPending, executionSlots,
		)
	}
}

// The pool reaches the dedupe gate concurrently rather than one caller at a
// time, and one message must still become exactly one turn.
func TestOneMessageBecomesOneTurnUnderABurst(t *testing.T) {
	t.Parallel()
	seen := newSeenMessages(1024)
	const racers = 64
	admitted := make(chan bool, racers)
	start := make(chan struct{})
	for range racers {
		go func() {
			<-start
			admitted <- seen.Add("one-message")
		}()
	}
	close(start)

	accepted := 0
	for range racers {
		if <-admitted {
			accepted++
		}
	}
	if accepted != 1 {
		t.Errorf("one message was admitted %d times, so a burst answers it more than once", accepted)
	}
}
