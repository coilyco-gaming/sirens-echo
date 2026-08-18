package community

import (
	"context"
	"errors"
	"strings"
	"testing"

	"go.temporal.io/sdk/client"
)

// startCall is one recorded SignalWithStartWorkflow.
type startCall struct {
	workflowID string
	signal     string
	arg        any
	options    client.StartWorkflowOptions
	workflow   any
}

type recordingStarter struct {
	calls []startCall
	err   error
}

func (s *recordingStarter) SignalWithStartWorkflow(
	_ context.Context,
	workflowID string,
	signalName string,
	signalArg any,
	options client.StartWorkflowOptions,
	target any,
	_ ...any,
) (client.WorkflowRun, error) {
	s.calls = append(s.calls, startCall{
		workflowID: workflowID, signal: signalName, arg: signalArg,
		options: options, workflow: target,
	})
	return nil, s.err
}

// One action per tool call, and the turn's trace is the key, so a turn's calls
// land in one ordered trajectory rather than one workflow each.
func TestEachToolCallIsOneSignalKeyedOnTheTurn(t *testing.T) {
	t.Parallel()
	starter := &recordingStarter{}
	mirror := temporalMirror{starter: starter, taskQueue: "sirens-deep"}

	for _, tool := range []string{"get_market", "find_trade"} {
		if err := mirror.MirrorToolCall(context.Background(), ToolCallRecord{
			Server: "eco", Tool: tool, Outcome: "ok", TraceID: "trace-1",
		}); err != nil {
			t.Fatalf("MirrorToolCall: %v", err)
		}
	}
	if err := mirror.MirrorToolCall(context.Background(), ToolCallRecord{
		Server: "eco", Tool: "get_market", Outcome: "ok", TraceID: "trace-2",
	}); err != nil {
		t.Fatalf("MirrorToolCall: %v", err)
	}

	if len(starter.calls) != 3 {
		t.Fatalf("%d actions against 3 tool calls", len(starter.calls))
	}
	if starter.calls[0].workflowID != starter.calls[1].workflowID {
		t.Errorf("one turn produced two workflows: %s and %s",
			starter.calls[0].workflowID, starter.calls[1].workflowID)
	}
	if starter.calls[1].workflowID == starter.calls[2].workflowID {
		t.Error("two turns shared one workflow, so the trajectories merged")
	}
	if !strings.Contains(starter.calls[0].workflowID, "trace-1") {
		t.Errorf("workflow id = %q, want the trace in it", starter.calls[0].workflowID)
	}
	if starter.calls[0].signal != TrajectorySignal ||
		starter.calls[0].workflow != TrajectoryWorkflow {
		t.Errorf("signal = %q workflow = %v",
			starter.calls[0].signal, starter.calls[0].workflow)
	}
	if starter.calls[0].options.TaskQueue != "sirens-deep" {
		t.Errorf("task queue = %q", starter.calls[0].options.TaskQueue)
	}
	// A trajectory that never sees a second call must not sit open forever.
	if starter.calls[0].options.WorkflowExecutionTimeout <= 0 {
		t.Error("no execution timeout, so an abandoned trajectory never ends")
	}
}

// The signal argument is the record itself, so widening the payload is a change
// to ToolCallRecord and nowhere else.
func TestTheSignalCarriesTheRecordUnchanged(t *testing.T) {
	t.Parallel()
	starter := &recordingStarter{}
	mirror := temporalMirror{starter: starter, taskQueue: "sirens-deep"}
	want := ToolCallRecord{
		Server: "eco", Tool: "get_market", Outcome: "tool_error",
		ElapsedMillis: 91, TraceID: "trace-1",
	}
	if err := mirror.MirrorToolCall(context.Background(), want); err != nil {
		t.Fatalf("MirrorToolCall: %v", err)
	}
	got, ok := starter.calls[0].arg.(ToolCallRecord)
	if !ok {
		t.Fatalf("signal argument is %T, want a ToolCallRecord", starter.calls[0].arg)
	}
	if got != want {
		t.Errorf("signalled %#v, want %#v", got, want)
	}
}

// A record with no trace has no key, and inventing one would merge unrelated
// turns into a single trajectory.
func TestARecordWithoutATraceIsRefusedRatherThanMerged(t *testing.T) {
	t.Parallel()
	starter := &recordingStarter{}
	mirror := temporalMirror{starter: starter, taskQueue: "sirens-deep"}
	if err := mirror.MirrorToolCall(context.Background(), ToolCallRecord{
		Server: "eco", Tool: "get_market",
	}); err == nil {
		t.Fatal("a record with no trace id was accepted")
	}
	if len(starter.calls) != 0 {
		t.Errorf("wrote %d actions anyway", len(starter.calls))
	}
}

// The client's error reaches the dispatch, which is what counts the drop.
func TestATemporalErrorIsReturnedToTheDispatch(t *testing.T) {
	t.Parallel()
	starter := &recordingStarter{err: errors.New("namespace unreachable")}
	mirror := temporalMirror{starter: starter, taskQueue: "sirens-deep"}
	if err := mirror.MirrorToolCall(context.Background(), ToolCallRecord{
		Server: "eco", Tool: "get_market", TraceID: "trace-1",
	}); err == nil {
		t.Fatal("a failed signal reported success")
	}
}

// A deployment with no client or no queue gets no mirror, rather than one that
// fails on every call.
func TestAnUnconfiguredTemporalMirrorIsNil(t *testing.T) {
	t.Parallel()
	if NewTemporalMirror(nil, "sirens-deep", "sirens-deep") != nil {
		t.Error("a nil client produced a mirror")
	}
	if NewTemporalMirror(nil, "", "sirens-deep") != nil {
		t.Error("an empty queue produced a mirror")
	}
}

// A half-filled connection is a typo, and a mirror that silently does nothing
// is the failure this design exists to avoid.
func TestAPartlyConfiguredMirrorIsRefused(t *testing.T) {
	t.Parallel()
	for name, cfg := range map[string]TemporalMirrorConfig{
		"no host":  {Namespace: "deep", TaskQueue: "q"},
		"no queue": {HostPort: "h:7233", Namespace: "deep"},
		"no space": {HostPort: "h:7233", TaskQueue: "q"},
	} {
		if err := cfg.Validate(); err == nil {
			t.Errorf("%s: a partial connection validated", name)
		}
	}
	// Nothing set is the packaged posture, not a misconfiguration.
	if err := (TemporalMirrorConfig{}).Validate(); err != nil {
		t.Errorf("an unconfigured mirror was refused: %v", err)
	}
	if _, mirror, err := DialTemporalMirror(TemporalMirrorConfig{}); err != nil || mirror != nil {
		t.Errorf("dialling an unconfigured mirror returned %v, %v", mirror, err)
	}
}

// Finding a turn should be pasting the message id a member already has, rather
// than reading a trace out of a failure notice first. See #977.
func TestTheWorkflowIDCarriesTheSummoningMessage(t *testing.T) {
	t.Parallel()
	starter := &recordingStarter{}
	mirror := temporalMirror{
		starter: starter, taskQueue: "sirens-dowel-tool-mirror", instance: "sirens-dowel",
	}
	if err := mirror.MirrorToolCall(context.Background(), ToolCallRecord{
		Server: "discord", Tool: "list_channel-message", Outcome: "ok",
		TraceID: "7c1d5e0f2331d19046769e7041a00181", RequestID: "1536447620116127784",
	}); err != nil {
		t.Fatalf("MirrorToolCall: %v", err)
	}
	want := "sirens-dowel-turn-1536447620116127784"
	if starter.calls[0].workflowID != want {
		t.Errorf("workflow id = %q, want %q", starter.calls[0].workflowID, want)
	}
	// The id the signal is keyed on and the id the workflow is started with are
	// one value, or a retry opens a second trajectory for the same turn.
	if starter.calls[0].options.ID != want {
		t.Errorf("options.ID = %q, want %q", starter.calls[0].options.ID, want)
	}
}

// The lane is in the id because one namespace holds every lane, and a bare
// message id would not say which agent answered it.
func TestTheWorkflowIDNamesTheLane(t *testing.T) {
	t.Parallel()
	for instance, want := range map[string]string{
		"sirens-dowel": "sirens-dowel-turn-42",
		"sirens-deep":  "sirens-deep-turn-42",
		"":             "sirens-turn-42",
	} {
		got := trajectoryID(instance, ToolCallRecord{RequestID: "42", TraceID: "t"})
		if got != want {
			t.Errorf("instance %q produced %q, want %q", instance, got, want)
		}
	}
}

// An HTTP turn carries no Discord message, so it keys on the trace and stays
// distinguishable from a message id rather than colliding with one.
func TestATurnWithNoRequestIDFallsBackToTheTrace(t *testing.T) {
	t.Parallel()
	got := trajectoryID("sirens-dowel", ToolCallRecord{TraceID: "7c1d5e0f"})
	want := "sirens-dowel-turn-trace-7c1d5e0f"
	if got != want {
		t.Errorf("workflow id = %q, want %q", got, want)
	}
}

// The id reaches the record through the context, so a turn that never attached
// one degrades to the trace rather than to a blank key.
func TestTheRequestIDRoundTripsThroughTheContext(t *testing.T) {
	t.Parallel()
	if got := RequestIDFromContext(context.Background()); got != "" {
		t.Errorf("a bare context carried %q", got)
	}
	ctx := ContextWithRequestID(context.Background(), "1536447620116127784")
	if got := RequestIDFromContext(ctx); got != "1536447620116127784" {
		t.Errorf("request id = %q", got)
	}
	// An empty id must not shadow an outer one with a blank value.
	if got := RequestIDFromContext(ContextWithRequestID(ctx, "")); got != "1536447620116127784" {
		t.Errorf("an empty id overwrote the turn's: %q", got)
	}
}
