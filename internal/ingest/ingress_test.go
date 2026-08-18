package ingest

import (
	"context"
	"sync"
	"testing"
	"time"
)

// recorder captures the order acknowledgments were applied in, which is the
// only thing that proves the ack happened before the queue could hold the ask.
type recorder struct {
	mu     sync.Mutex
	queued []int64
	shed   []int64
}

func (r *recorder) Queued(_ context.Context, ask Ask) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.queued = append(r.queued, ask.Seq)
	return nil
}

func (r *recorder) Shed(_ context.Context, ask Ask) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.shed = append(r.shed, ask.Seq)
	return nil
}

func tenant(author string) Tenant {
	return Tenant{Surface: SurfaceDiscord, Guild: "g", Channel: "c", Author: author}
}

type subject string

func (s subject) ID() string { return string(s) }

func TestEveryAskIsAcknowledgedBeforeItIsQueued(t *testing.T) {
	t.Parallel()
	acks := &recorder{}
	queue := NewQueue(8)
	in := NewIngress(queue, acks, nil, nil, nil)
	for i := 0; i < 5; i++ {
		in.Submit(context.Background(), tenant("ana"), "thread", "hello", subject("m"))
	}
	if len(acks.queued) != 5 {
		t.Fatalf("acknowledged %d asks, want one per comment", len(acks.queued))
	}
	// The ask is in the queue only after its ack returned, so an ack recorded
	// at depth d must have been the (d+1)th, never one the coalescer had seen.
	for i, seq := range acks.queued {
		if seq != int64(i+1) {
			t.Fatalf("ack %d carried seq %d, want %d", i, seq, i+1)
		}
	}
}

func TestSequenceIsMonotonicAcrossConcurrentSubmits(t *testing.T) {
	t.Parallel()
	in := NewIngress(NewQueue(256), nil, nil, nil, nil)
	var wg sync.WaitGroup
	seen := make([]int64, 100)
	for i := 0; i < 100; i++ {
		wg.Add(1)
		go func(at int) {
			defer wg.Done()
			seen[at] = in.Submit(context.Background(), tenant("ana"), "t", "q", subject("m")).Seq
		}(i)
	}
	wg.Wait()
	unique := make(map[int64]bool, len(seen))
	for _, seq := range seen {
		if seq < 1 || seq > 100 {
			t.Fatalf("sequence %d outside the range assigned", seq)
		}
		if unique[seq] {
			t.Fatalf("sequence %d was assigned twice", seq)
		}
		unique[seq] = true
	}
}

func TestOverflowShedsTheOldestAndRetractsItsMark(t *testing.T) {
	t.Parallel()
	acks := &recorder{}
	queue := NewQueue(3)
	in := NewIngress(queue, acks, nil, nil, nil)
	for i := 0; i < 5; i++ {
		in.Submit(context.Background(), tenant("ana"), "t", "q", subject("m"))
	}
	if len(acks.shed) != 2 {
		t.Fatalf("shed %d asks, want the 2 the bound had no room for", len(acks.shed))
	}
	if acks.shed[0] != 1 || acks.shed[1] != 2 {
		t.Fatalf("shed %v, want the oldest two", acks.shed)
	}
	// A shed ask keeping its queued mark is a promise the service cannot keep,
	// so the retraction is the part that matters rather than the log line.
	if len(acks.queued) != 5 {
		t.Fatalf("acknowledged %d, want every comment marked on arrival", len(acks.queued))
	}
	remaining := []int64{3, 4, 5}
	for _, want := range remaining {
		ask, ok := queue.TryNext()
		if !ok || ask.Seq != want {
			t.Fatalf("queue held %v, want %d", ask.Seq, want)
		}
	}
}

func TestIngressNeverBlocksOnAFullQueue(t *testing.T) {
	t.Parallel()
	in := NewIngress(NewQueue(2), nil, nil, nil, nil)
	done := make(chan struct{})
	go func() {
		defer close(done)
		for i := 0; i < 1000; i++ {
			in.Submit(context.Background(), tenant("ana"), "t", "q", subject("m"))
		}
	}()
	select {
	case <-done:
	case <-time.After(5 * time.Second):
		t.Fatal("ingress blocked with nothing draining the queue")
	}
}

func TestClosedQueueDrainsBeforeItStops(t *testing.T) {
	t.Parallel()
	queue := NewQueue(8)
	in := NewIngress(queue, nil, nil, nil, nil)
	in.Submit(context.Background(), tenant("ana"), "t", "q", subject("m"))
	in.Submit(context.Background(), tenant("bo"), "t", "q", subject("m"))
	queue.Close()
	for want := int64(1); want <= 2; want++ {
		ask, ok := queue.Next(context.Background())
		if !ok || ask.Seq != want {
			t.Fatalf("close dropped ask %d", want)
		}
	}
	if _, ok := queue.Next(context.Background()); ok {
		t.Fatal("a drained closed queue kept returning asks")
	}
}

func TestTenantKeyShardsOnTheMemberInsideAChannel(t *testing.T) {
	t.Parallel()
	ana := tenant("ana")
	bo := tenant("bo")
	if ana.Key() == bo.Key() {
		t.Fatal("two members in one channel share a shard, so they serialize")
	}
	elsewhere := ana
	elsewhere.Channel = "other"
	if ana.Key() == elsewhere.Key() {
		t.Fatal("one member in two channels shares a shard")
	}
}
