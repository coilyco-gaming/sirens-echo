package community

import (
	"testing"
	"time"

	"go.temporal.io/sdk/testsuite"
)

// The trajectory had no worker, so it ran to its lifetime ceiling and retired
// as TimedOut. Nothing had ever asserted that it completes. sirens-echo#1041.

func TestToolTrajectoryCompletesOnceTheCallsStop(t *testing.T) {
	var suite testsuite.WorkflowTestSuite
	env := suite.NewTestWorkflowEnvironment()
	env.RegisterWorkflow(ToolTrajectoryWorkflow)

	env.RegisterDelayedCallback(func() {
		env.SignalWorkflow(TrajectorySignal, ToolCallRecord{
			Server: "forgejo", Tool: "list_issue", TraceID: "trace-1", RequestID: "req-1",
		})
	}, time.Second)
	env.RegisterDelayedCallback(func() {
		env.SignalWorkflow(TrajectorySignal, ToolCallRecord{
			Server: "exa", Tool: "create_web_search", TraceID: "trace-1", RequestID: "req-1",
		})
	}, 2*time.Second)

	env.ExecuteWorkflow(ToolTrajectoryWorkflow)

	if !env.IsWorkflowCompleted() {
		t.Fatal("the trajectory never completed, which is what leaves it TimedOut")
	}
	if err := env.GetWorkflowError(); err != nil {
		t.Fatalf("trajectory errored: %v", err)
	}
	var records []ToolCallRecord
	if err := env.GetWorkflowResult(&records); err != nil {
		t.Fatalf("result: %v", err)
	}
	if len(records) != 2 {
		t.Fatalf("kept %d records, want both signals", len(records))
	}
	if records[0].Tool != "list_issue" || records[1].Tool != "create_web_search" {
		t.Fatalf("records arrived out of order: %+v", records)
	}
}

// A turn that calls nothing further must still close rather than sit open.
func TestToolTrajectoryClosesWithoutASecondCall(t *testing.T) {
	var suite testsuite.WorkflowTestSuite
	env := suite.NewTestWorkflowEnvironment()
	env.RegisterWorkflow(ToolTrajectoryWorkflow)

	env.RegisterDelayedCallback(func() {
		env.SignalWorkflow(TrajectorySignal, ToolCallRecord{
			Server: "discord", Tool: "list_channel-message", TraceID: "trace-2",
		})
	}, time.Second)

	env.ExecuteWorkflow(ToolTrajectoryWorkflow)

	if !env.IsWorkflowCompleted() {
		t.Fatal("a single-call trajectory stayed open")
	}
	if err := env.GetWorkflowError(); err != nil {
		t.Fatalf("trajectory errored: %v", err)
	}
}
