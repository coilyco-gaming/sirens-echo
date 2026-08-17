package community

import (
	"context"
	"encoding/json"
	"errors"
	"reflect"
	"strings"
	"sync"
	"testing"
	"time"

	"go.opentelemetry.io/otel/attribute"
)

// The acceptance on sirens-echo#887 is the spec these pin.

// recordingMirror captures what the dispatch delivered.
type recordingMirror struct {
	mu       sync.Mutex
	records  []ToolCallRecord
	err      error
	block    chan struct{}
	panicNow bool
	arrived  chan struct{}
}

func newRecordingMirror() *recordingMirror {
	return &recordingMirror{arrived: make(chan struct{}, 64)}
}

func (m *recordingMirror) MirrorToolCall(_ context.Context, record ToolCallRecord) error {
	if m.panicNow {
		panic("a third-party client blew up")
	}
	if m.block != nil {
		<-m.block
	}
	m.mu.Lock()
	m.records = append(m.records, record)
	m.mu.Unlock()
	select {
	case m.arrived <- struct{}{}:
	default:
	}
	return m.err
}

func (m *recordingMirror) seen() []ToolCallRecord {
	m.mu.Lock()
	defer m.mu.Unlock()
	return append([]ToolCallRecord(nil), m.records...)
}

func (m *recordingMirror) waitFor(t *testing.T, count int) {
	t.Helper()
	for range count {
		select {
		case <-m.arrived:
		case <-time.After(3 * time.Second):
			t.Fatalf("only %d records arrived, want %d", len(m.seen()), count)
		}
	}
}

// Acceptance 1: a tool call produces a record carrying the four fields.
func TestAToolCallIsMirroredWithItsMetadata(t *testing.T) {
	t.Parallel()
	mirror := newRecordingMirror()
	dispatch := newMirrorDispatch(mirror, 8, time.Second, nil)
	defer dispatch.Close()

	dispatch.send(ToolCallRecord{
		Server: "eco", Tool: "get_market", Outcome: "ok",
		ElapsedMillis: 42, TraceID: "abc123",
	})
	mirror.waitFor(t, 1)

	got := mirror.seen()[0]
	want := ToolCallRecord{
		Server: "eco", Tool: "get_market", Outcome: "ok",
		ElapsedMillis: 42, TraceID: "abc123",
	}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("record = %#v, want %#v", got, want)
	}
}

// Acceptance 2: the payload is metadata. This is the test that has to fail if
// someone widens the struct, so it enumerates the fields rather than sampling.
func TestTheMirroredPayloadCarriesNothingButMetadata(t *testing.T) {
	t.Parallel()
	shape := reflect.TypeOf(ToolCallRecord{})
	allowed := map[string]bool{
		"Server": true, "Tool": true, "Outcome": true,
		"ElapsedMillis": true, "TraceID": true,
	}
	for index := range shape.NumField() {
		name := shape.Field(index).Name
		if !allowed[name] {
			t.Errorf("ToolCallRecord gained %q. Every field here leaves this "+
				"process, so widening it is a disclosure decision", name)
		}
	}
	if shape.NumField() != len(allowed) {
		t.Errorf("field count = %d, want %d", shape.NumField(), len(allowed))
	}

	// And the encoded form, because that is what actually travels.
	encoded, err := json.Marshal(ToolCallRecord{
		Server: "eco", Tool: "get_market", Outcome: "ok",
		ElapsedMillis: 7, TraceID: "abc123",
	})
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	for _, forbidden := range []string{
		"content", "message", "argument", "result", "text", "principal",
		"user", "author", "prompt", "reply",
	} {
		if strings.Contains(strings.ToLower(string(encoded)), forbidden) {
			t.Errorf("payload %s contains %q", encoded, forbidden)
		}
	}
}

// Acceptance 5: the mirror reads no span attributes, so a content-bearing one
// added to any span changes nothing about what leaves.
func TestAContentBearingSpanAttributeDoesNotReachTheMirror(t *testing.T) {
	t.Parallel()
	telemetry := telemetryOrNoop(nil)
	mirror := newRecordingMirror()
	telemetry.AttachToolMirror(mirror)
	defer telemetry.CloseToolMirror()

	// Exactly the failure the issue names: someone adds member text to a span.
	ctx, span := telemetry.StartSpan(
		context.Background(),
		"mcp.tool.call",
		attribute.String("message.text", "a member said something private"),
		attribute.String("mcp.tool.arguments", `{"secret":"value"}`),
	)
	defer span.End()

	telemetry.RecordToolCall(ctx, "eco", "get_market", "ok", 5*time.Millisecond)
	mirror.waitFor(t, 1)

	encoded, err := json.Marshal(mirror.seen()[0])
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	for _, leaked := range []string{"private", "secret", "message.text", "arguments"} {
		if strings.Contains(string(encoded), leaked) {
			t.Fatalf("the span attribute reached the mirror: %s", encoded)
		}
	}
}

// Acceptance 4: one record per tool call, not one per span. StartSpan is called
// many times per turn and only RecordToolCall mirrors.
func TestOnlyToolCallsAreMirrored(t *testing.T) {
	t.Parallel()
	telemetry := telemetryOrNoop(nil)
	mirror := newRecordingMirror()
	telemetry.AttachToolMirror(mirror)
	defer telemetry.CloseToolMirror()

	ctx := context.Background()
	for _, name := range []string{"community.turn", "community.input", "model.chat"} {
		_, span := telemetry.StartSpan(ctx, name)
		span.End()
	}
	telemetry.RecordTurn(ctx, "ok", time.Second)
	telemetry.RecordModelCall(ctx, "ok")
	telemetry.RecordFailure(ctx, "model")

	telemetry.RecordToolCall(ctx, "eco", "get_market", "ok", time.Millisecond)
	telemetry.RecordToolCall(ctx, "eco", "find_trade", "ok", time.Millisecond)
	mirror.waitFor(t, 2)

	if got := len(mirror.seen()); got != 2 {
		t.Errorf("mirrored %d records against 2 tool calls and six other events", got)
	}
}

// Acceptance 3, half one: a mirror that fails does not reach the caller, and
// the drop is counted.
func TestAFailingMirrorIsCountedAndNeverReturned(t *testing.T) {
	t.Parallel()
	mirror := newRecordingMirror()
	mirror.err = errors.New("temporal is down")
	drops := make(chan struct{}, 8)
	dispatch := newMirrorDispatch(mirror, 4, time.Second, func() { drops <- struct{}{} })
	defer dispatch.Close()

	dispatch.send(ToolCallRecord{Server: "eco", Tool: "get_market", TraceID: "t"})
	select {
	case <-drops:
	case <-time.After(3 * time.Second):
		t.Fatal("a failed mirror was not counted, so the outage is silent")
	}
}

// A panicking client is the other way a third-party SDK ends a process.
func TestAPanickingMirrorIsContained(t *testing.T) {
	t.Parallel()
	mirror := newRecordingMirror()
	mirror.panicNow = true
	drops := make(chan struct{}, 8)
	dispatch := newMirrorDispatch(mirror, 4, time.Second, func() { drops <- struct{}{} })
	defer dispatch.Close()

	dispatch.send(ToolCallRecord{Server: "eco", Tool: "get_market", TraceID: "t"})
	select {
	case <-drops:
	case <-time.After(3 * time.Second):
		t.Fatal("a panicking mirror was not contained")
	}
}

// Acceptance 3, half two: an induced outage leaves the turn unaffected. The
// send must not block even with every worker stuck and the queue full.
func TestAStalledMirrorNeverBlocksTheTurn(t *testing.T) {
	t.Parallel()
	mirror := newRecordingMirror()
	mirror.block = make(chan struct{})
	defer close(mirror.block)

	var dropped int
	var mu sync.Mutex
	dispatch := newMirrorDispatch(mirror, 2, time.Second, func() {
		mu.Lock()
		dropped++
		mu.Unlock()
	})

	// Far more than the queue holds, with the only worker wedged.
	done := make(chan struct{})
	go func() {
		defer close(done)
		for range 500 {
			dispatch.send(ToolCallRecord{Server: "eco", Tool: "t", TraceID: "x"})
		}
	}()
	select {
	case <-done:
	case <-time.After(3 * time.Second):
		t.Fatal("sending blocked behind a stalled mirror, so a turn would too")
	}
	mu.Lock()
	defer mu.Unlock()
	if dropped == 0 {
		t.Error("nothing was counted as dropped, so the loss is invisible")
	}
}

// A deployment without a mirror runs the nil path, which must stay a no-op
// rather than a nil dereference on the hot path.
func TestNoMirrorIsANoOp(t *testing.T) {
	t.Parallel()
	if dispatch := newMirrorDispatch(nil, 8, time.Second, nil); dispatch != nil {
		t.Fatal("a nil mirror produced a dispatch")
	}
	var dispatch *mirrorDispatch
	dispatch.send(ToolCallRecord{Server: "eco"})
	dispatch.Close()

	telemetry := telemetryOrNoop(nil)
	telemetry.RecordToolCall(context.Background(), "eco", "get_market", "ok", time.Second)
}
