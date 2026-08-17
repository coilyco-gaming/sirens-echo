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
	if NewTemporalMirror(nil, "sirens-deep") != nil {
		t.Error("a nil client produced a mirror")
	}
	if NewTemporalMirror(nil, "") != nil {
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
