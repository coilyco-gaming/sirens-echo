package community

import (
	"context"
	"fmt"
	"strings"

	"go.temporal.io/sdk/client"
	"go.temporal.io/sdk/workflow"
)

// The Temporal Cloud end of the mirror. See docs/sirens-echo-tool-markup.md.

const (
	// TrajectoryWorkflow is the workflow type one turn's tool calls land in.
	// Lane-neutral: one type serves every lane and the id says which.
	TrajectoryWorkflow = "ToolTrajectory"
	// TrajectorySignal carries one ToolCallRecord.
	TrajectorySignal = "tool-call"
)

// temporalStarter is the one client call this needs, so a test substitutes it.
type temporalStarter interface {
	SignalWithStartWorkflow(
		ctx context.Context,
		workflowID string,
		signalName string,
		signalArg any,
		options client.StartWorkflowOptions,
		workflow any,
		workflowArgs ...any,
	) (client.WorkflowRun, error)
}

// temporalMirror signals one workflow per turn, keyed on the trace id, so the
// trajectory arrives ordered and costs one action per tool call.
type temporalMirror struct {
	starter   temporalStarter
	taskQueue string
	instance  string
}

// NewTemporalMirror builds the mirror over a connected client. The instance
// names the lane in every workflow id, so one namespace can hold several.
func NewTemporalMirror(temporal client.Client, taskQueue, instance string) ToolCallMirror {
	if temporal == nil || strings.TrimSpace(taskQueue) == "" {
		return nil
	}
	return temporalMirror{starter: temporal, taskQueue: taskQueue, instance: instance}
}

// trajectoryID keys the turn on the thing a human already has. On Discord that
// is the summoning message id, so finding a turn is pasting it into the id.
func trajectoryID(instance string, record ToolCallRecord) string {
	lane := strings.TrimSpace(instance)
	if lane == "" {
		lane = "sirens"
	}
	if request := strings.TrimSpace(record.RequestID); request != "" {
		return lane + "-turn-" + request
	}
	// A turn with no request id is still one trajectory, so it keys on the
	// trace and stays distinguishable from a message id at a glance.
	return lane + "-turn-trace-" + record.TraceID
}

// MirrorToolCall records one call. SignalWithStart is one action whether the
// turn's workflow exists yet or not, which is what bounds the volume.
func (m temporalMirror) MirrorToolCall(ctx context.Context, record ToolCallRecord) error {
	if strings.TrimSpace(record.TraceID) == "" {
		return fmt.Errorf("tool call record carries no trace id")
	}
	id := trajectoryID(m.instance, record)
	_, err := m.starter.SignalWithStartWorkflow(
		ctx,
		id,
		TrajectorySignal,
		record,
		client.StartWorkflowOptions{
			ID:        id,
			TaskQueue: m.taskQueue,
			// The turn is over long before this. A trajectory that never sees a
			// second call must not sit open forever.
			WorkflowExecutionTimeout: trajectoryLifetime,
		},
		TrajectoryWorkflow,
	)
	return err
}

// ToolTrajectoryWorkflow accumulates one turn's records and ends when they stop.
// It performs no activity. See docs/sirens-echo-tool-markup.md.
func ToolTrajectoryWorkflow(ctx workflow.Context) ([]ToolCallRecord, error) {
	records := make([]ToolCallRecord, 0, 8)
	signals := workflow.GetSignalChannel(ctx, TrajectorySignal)
	for {
		var record ToolCallRecord
		arrived := false
		selector := workflow.NewSelector(ctx)
		selector.AddReceive(signals, func(channel workflow.ReceiveChannel, _ bool) {
			channel.Receive(ctx, &record)
			arrived = true
		})
		selector.AddFuture(workflow.NewTimer(ctx, trajectoryIdle), func(workflow.Future) {})
		selector.Select(ctx)
		if !arrived {
			// The idle timer won, so the turn has stopped calling tools.
			return records, nil
		}
		records = append(records, record)
	}
}

// TemporalMirrorConfig is what a deployment supplies to turn the mirror on.
// Every field empty means off, which is the packaged posture.
type TemporalMirrorConfig struct {
	HostPort  string
	Namespace string
	TaskQueue string
	// APIKey is read from the pod environment and never logged. Provisioning is
	// sirens-echo#444.
	APIKey string
	// Instance names the lane in every workflow id. Not part of the connection,
	// so a missing one is not a half-filled config.
	Instance string
}

// configured reports a connection worth dialling. A partial one is a
// misconfiguration rather than a request to run without a mirror.
func (c TemporalMirrorConfig) configured() bool {
	return c.HostPort != "" || c.Namespace != "" || c.TaskQueue != ""
}

// Validate refuses a half-filled connection, so a typo turns the mirror off
// loudly at boot rather than quietly at the first tool call.
func (c TemporalMirrorConfig) Validate() error {
	if !c.configured() {
		return nil
	}
	for name, value := range map[string]string{
		"SIRENS_ECHO_TEMPORAL_HOST":       c.HostPort,
		"SIRENS_ECHO_TEMPORAL_NAMESPACE":  c.Namespace,
		"SIRENS_ECHO_TEMPORAL_TASK_QUEUE": c.TaskQueue,
	} {
		if strings.TrimSpace(value) == "" {
			return fmt.Errorf("the Temporal mirror is partly configured and %s is empty", name)
		}
	}
	return nil
}

// DialTemporalMirror connects, or returns nil when no mirror is configured.
// A dial failure is returned rather than swallowed.
func DialTemporalMirror(cfg TemporalMirrorConfig) (client.Client, ToolCallMirror, error) {
	if err := cfg.Validate(); err != nil {
		return nil, nil, err
	}
	if !cfg.configured() {
		return nil, nil, nil
	}
	options := client.Options{HostPort: cfg.HostPort, Namespace: cfg.Namespace}
	if cfg.APIKey != "" {
		options.Credentials = client.NewAPIKeyStaticCredentials(cfg.APIKey)
	}
	temporal, err := client.Dial(options)
	if err != nil {
		return nil, nil, fmt.Errorf("dial Temporal: %w", err)
	}
	return temporal, NewTemporalMirror(temporal, cfg.TaskQueue, cfg.Instance), nil
}
