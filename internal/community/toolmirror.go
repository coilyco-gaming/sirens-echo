package community

import (
	"context"
	"log/slog"
	"time"
)

// Mirroring the tool-call trajectory, metadata only. Temporal observes the
// turn, it never runs it. See docs/sirens-echo-tool-markup.md.

// ToolCallRecord is the entire mirrored payload. Named fields rather than an
// attribute slice, and the reason is docs/sirens-echo-tool-markup.md.
type ToolCallRecord struct {
	// Server and Tool are roster identifiers this repository declares.
	Server string `json:"server"`
	Tool   string `json:"tool"`
	// Outcome is the closed set the metric already uses.
	Outcome string `json:"outcome"`
	// ElapsedMillis is the call's own duration, not the turn's.
	ElapsedMillis int64 `json:"elapsed_millis"`
	// TraceID correlates the record with the trace. It names no person, and a
	// member is already handed one in a failure notice.
	TraceID string `json:"trace_id"`
}

// ToolCallMirror receives one record. Implementations must be safe to call
// from a worker goroutine and must not retain the context.
type ToolCallMirror interface {
	MirrorToolCall(ctx context.Context, record ToolCallRecord) error
}

// mirrorDispatch keeps the mirror off the hot path. A full queue drops rather
// than blocks, because a turn must never wait on an audit record.
type mirrorDispatch struct {
	records chan ToolCallRecord
	mirror  ToolCallMirror
	timeout time.Duration
	dropped func()
	stop    chan struct{}
	done    chan struct{}
}

// newMirrorDispatch starts the worker. A nil mirror returns nil, so every call
// site stays a nil check rather than a feature flag.
func newMirrorDispatch(
	mirror ToolCallMirror,
	depth int,
	timeout time.Duration,
	dropped func(),
) *mirrorDispatch {
	if mirror == nil {
		return nil
	}
	if depth < 1 {
		depth = 1
	}
	dispatch := &mirrorDispatch{
		records: make(chan ToolCallRecord, depth),
		mirror:  mirror,
		timeout: timeout,
		dropped: dropped,
		stop:    make(chan struct{}),
		done:    make(chan struct{}),
	}
	go dispatch.work()
	return dispatch
}

// send never blocks. A dropped record is counted, so the silence is measurable
// rather than invisible.
func (d *mirrorDispatch) send(record ToolCallRecord) {
	if d == nil {
		return
	}
	select {
	case d.records <- record:
	default:
		d.drop()
	}
}

func (d *mirrorDispatch) drop() {
	if d.dropped != nil {
		d.dropped()
	}
}

// work owns the only call into the mirror. Errors are counted and swallowed:
// an audit backend being down is not a reason for a turn to fail.
func (d *mirrorDispatch) work() {
	defer close(d.done)
	for {
		select {
		case <-d.stop:
			return
		case record := <-d.records:
			d.deliver(record)
		}
	}
}

func (d *mirrorDispatch) deliver(record ToolCallRecord) {
	// Detached from the turn, so a finished turn does not cancel its own audit
	// record, and hard-bounded so a hung backend cannot stall the queue.
	ctx, cancel := context.WithTimeout(context.Background(), d.timeout)
	defer cancel()
	defer func() {
		// A panicking third-party client must not take the process with it.
		if recovered := recover(); recovered != nil {
			d.drop()
		}
	}()
	if err := d.mirror.MirrorToolCall(ctx, record); err != nil {
		d.drop()
	}
}

// Close drains nothing and waits for the in-flight record only. Anything still
// queued at shutdown is dropped by design, because the turn already answered.
func (d *mirrorDispatch) Close() {
	if d == nil {
		return
	}
	close(d.stop)
	<-d.done
}

// attachToolMirror connects the mirror when one is configured. A typo is fatal
// and an outage is not. See docs/sirens-echo-tool-markup.md.
func (a *Agent) attachToolMirror() error {
	if err := a.cfg.TemporalMirror.Validate(); err != nil {
		return err
	}
	temporal, mirror, err := DialTemporalMirror(a.cfg.TemporalMirror)
	if err != nil {
		a.telemetry.Error(context.Background(), "mirror.dial.failed",
			slog.String("error", err.Error()))
		return nil
	}
	if mirror == nil {
		return nil
	}
	a.temporal = temporal
	a.telemetry.AttachToolMirror(mirror)
	a.telemetry.Info(context.Background(), "mirror.attached",
		slog.String("namespace", a.cfg.TemporalMirror.Namespace),
		slog.String("task_queue", a.cfg.TemporalMirror.TaskQueue))
	return nil
}

// closeToolMirror stops the worker and the client, in that order, so nothing
// is delivered into a closed connection.
func (a *Agent) closeToolMirror() {
	a.telemetry.CloseToolMirror()
	if a.temporal != nil {
		a.temporal.Close()
	}
}
