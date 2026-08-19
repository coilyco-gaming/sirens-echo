// Package temporalmcp serves the lane's own Temporal namespace as read-only MCP
// tools. See deploy#698.
package temporalmcp

import (
	"context"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/modelcontextprotocol/go-sdk/mcp"
	enumspb "go.temporal.io/api/enums/v1"
	workflowpb "go.temporal.io/api/workflow/v1"
	"go.temporal.io/api/workflowservice/v1"
	"go.temporal.io/sdk/client"
)

// Path is where the handler is mounted, matching what a roster entry names.
const Path = "/mcp"

const defaultPageSize = 20

const maxPageSize = 100

// Config is the connection, spelled with the same environment variables the
// tool-call mirror already reads so one credential serves both directions.
type Config struct {
	HostPort  string
	Namespace string
	APIKey    string
	Instance  string
}

// Validate refuses a half-filled connection, so a typo fails at boot loudly
// rather than at the first tool call.
func (c Config) Validate() error {
	for name, value := range map[string]string{
		"SIRENS_ECHO_TEMPORAL_HOST":      c.HostPort,
		"SIRENS_ECHO_TEMPORAL_NAMESPACE": c.Namespace,
	} {
		if strings.TrimSpace(value) == "" {
			return fmt.Errorf("%s is required", name)
		}
	}
	return nil
}

// Dial opens the read client. The namespace is fixed here rather than taken per
// call, so no tool argument can reach a namespace this lane was not given.
func Dial(cfg Config) (client.Client, error) {
	if err := cfg.Validate(); err != nil {
		return nil, err
	}
	options := client.Options{HostPort: cfg.HostPort, Namespace: cfg.Namespace}
	if cfg.APIKey != "" {
		options.Credentials = client.NewAPIKeyStaticCredentials(cfg.APIKey)
	}
	temporal, err := client.Dial(options)
	if err != nil {
		return nil, fmt.Errorf("dial Temporal: %w", err)
	}
	return temporal, nil
}

// ListInput selects executions with a Temporal visibility query.
type ListInput struct {
	Query    string `json:"query,omitempty"`
	PageSize int    `json:"page_size,omitempty"`
}

// Execution is one workflow, flattened. The SDK returns protobuf whose JSON is
// mostly envelope, and a model reading a tool result pays for every byte.
type Execution struct {
	WorkflowID string `json:"workflow_id"`
	RunID      string `json:"run_id"`
	Type       string `json:"type"`
	Status     string `json:"status"`
	TaskQueue  string `json:"task_queue,omitempty"`
	StartTime  string `json:"start_time,omitempty"`
	CloseTime  string `json:"close_time,omitempty"`
}

// ListOutput is the page. No cursor is offered, because a lane that needs one
// wants a narrower query instead.
type ListOutput struct {
	Executions []Execution `json:"executions"`
	Count      int         `json:"count"`
}

// DescribeInput names one execution. An empty run id means the latest run.
type DescribeInput struct {
	WorkflowID string `json:"workflow_id"`
	RunID      string `json:"run_id,omitempty"`
}

// DescribeOutput adds what a list row cannot carry.
type DescribeOutput struct {
	Execution        Execution `json:"execution"`
	PendingActivity  int       `json:"pending_activities"`
	PendingChildren  int       `json:"pending_children"`
	HistoryLength    int64     `json:"history_length"`
	ParentWorkflowID string    `json:"parent_workflow_id,omitempty"`
}

// HistoryInput walks one execution's events.
type HistoryInput struct {
	WorkflowID string `json:"workflow_id"`
	RunID      string `json:"run_id,omitempty"`
	Limit      int    `json:"limit,omitempty"`
}

// Event is one history entry, named and timed. The payloads are deliberately
// absent: they carry tool arguments, and this lane mirrors metadata only.
type Event struct {
	EventID int64  `json:"event_id"`
	Type    string `json:"type"`
	Time    string `json:"time,omitempty"`
}

// HistoryOutput is the walked prefix, with truncation stated rather than implied.
type HistoryOutput struct {
	Events    []Event `json:"events"`
	Count     int     `json:"count"`
	Truncated bool    `json:"truncated"`
}

// Handler mounts the three read tools. Nothing that mutates a workflow is
// registered, so deny-by-absence is the whole enforcement.
func Handler(temporal client.Client, cfg Config) http.Handler {
	server := mcp.NewServer(
		&mcp.Implementation{
			Name:    strings.TrimSpace(cfg.Instance) + "-temporal",
			Title:   "Temporal, read-only",
			Version: "1",
		},
		&mcp.ServerOptions{Instructions: instructions(cfg)},
	)
	readOnly := &mcp.ToolAnnotations{ReadOnlyHint: true}
	s := &surface{temporal: temporal, namespace: cfg.Namespace}
	mcp.AddTool(server, &mcp.Tool{
		Name:        "list_workflows",
		Annotations: readOnly,
		Description: "List workflow executions in this deployment's own Temporal " +
			"namespace, newest first. Takes an optional Temporal visibility query " +
			"such as WorkflowType='ToolTrajectory' or " +
			"ExecutionStatus='Running'. Returns one row per execution with its id, " +
			"run id, type, status and times. Use this to find an execution before " +
			"describing it or reading its history.",
	}, s.list)
	mcp.AddTool(server, &mcp.Tool{
		Name:        "describe_workflow",
		Annotations: readOnly,
		Description: "Describe one workflow execution: its status, times, history " +
			"length, and how much work is still pending. Needs a workflow id from " +
			"list_workflows. An absent run id means the latest run.",
	}, s.describe)
	mcp.AddTool(server, &mcp.Tool{
		Name:        "get_workflow_history",
		Annotations: readOnly,
		Description: "Read one execution's history as a list of event ids, types " +
			"and times. Event payloads are not returned. Use this to see what a " +
			"workflow actually did and in what order, after describe_workflow says " +
			"how long the history is.",
	}, s.history)
	return mcp.NewStreamableHTTPHandler(
		func(*http.Request) *mcp.Server { return server },
		&mcp.StreamableHTTPOptions{},
	)
}

func instructions(cfg Config) string {
	return fmt.Sprintf(
		"Read-only access to the Temporal namespace %q, where this deployment "+
			"mirrors its own tool calls. Every tool here observes and none of "+
			"them starts, signals, cancels or terminates anything. Event "+
			"payloads are never returned, because the mirror records metadata "+
			"only.",
		cfg.Namespace,
	)
}

type surface struct {
	temporal  client.Client
	namespace string
}

func (s *surface) list(
	ctx context.Context,
	_ *mcp.CallToolRequest,
	input ListInput,
) (*mcp.CallToolResult, ListOutput, error) {
	size := input.PageSize
	if size <= 0 {
		size = defaultPageSize
	}
	if size > maxPageSize {
		size = maxPageSize
	}
	response, err := s.temporal.ListWorkflow(ctx, &workflowservice.ListWorkflowExecutionsRequest{
		Namespace: s.namespace,
		PageSize:  int32(size),
		Query:     strings.TrimSpace(input.Query),
	})
	if err != nil {
		return failure(err), ListOutput{}, nil
	}
	out := ListOutput{Executions: make([]Execution, 0, len(response.GetExecutions()))}
	for _, info := range response.GetExecutions() {
		out.Executions = append(out.Executions, execution(info))
	}
	out.Count = len(out.Executions)
	return nil, out, nil
}

func (s *surface) describe(
	ctx context.Context,
	_ *mcp.CallToolRequest,
	input DescribeInput,
) (*mcp.CallToolResult, DescribeOutput, error) {
	if strings.TrimSpace(input.WorkflowID) == "" {
		return failureText("workflow_id is required"), DescribeOutput{}, nil
	}
	response, err := s.temporal.DescribeWorkflowExecution(ctx, input.WorkflowID, input.RunID)
	if err != nil {
		return failure(err), DescribeOutput{}, nil
	}
	info := response.GetWorkflowExecutionInfo()
	out := DescribeOutput{
		Execution:       execution(info),
		PendingActivity: len(response.GetPendingActivities()),
		PendingChildren: len(response.GetPendingChildren()),
		HistoryLength:   info.GetHistoryLength(),
	}
	if parent := info.GetParentExecution(); parent != nil {
		out.ParentWorkflowID = parent.GetWorkflowId()
	}
	return nil, out, nil
}

func (s *surface) history(
	ctx context.Context,
	_ *mcp.CallToolRequest,
	input HistoryInput,
) (*mcp.CallToolResult, HistoryOutput, error) {
	if strings.TrimSpace(input.WorkflowID) == "" {
		return failureText("workflow_id is required"), HistoryOutput{}, nil
	}
	limit := input.Limit
	if limit <= 0 {
		limit = defaultPageSize
	}
	if limit > maxPageSize {
		limit = maxPageSize
	}
	// isLongPoll false, so an open workflow returns what exists rather than
	// blocking this request until the next event.
	iter := s.temporal.GetWorkflowHistory(
		ctx, input.WorkflowID, input.RunID, false,
		enumspb.HISTORY_EVENT_FILTER_TYPE_ALL_EVENT,
	)
	out := HistoryOutput{Events: make([]Event, 0, limit)}
	for iter.HasNext() {
		if len(out.Events) == limit {
			out.Truncated = true
			break
		}
		event, err := iter.Next()
		if err != nil {
			return failure(err), HistoryOutput{}, nil
		}
		entry := Event{EventID: event.GetEventId(), Type: event.GetEventType().String()}
		if stamp := event.GetEventTime(); stamp != nil {
			entry.Time = stamp.AsTime().UTC().Format(time.RFC3339)
		}
		out.Events = append(out.Events, entry)
	}
	out.Count = len(out.Events)
	return nil, out, nil
}

// execution flattens the one protobuf shape both list and describe return.
func execution(info *workflowpb.WorkflowExecutionInfo) Execution {
	out := Execution{
		Type:      info.GetType().GetName(),
		Status:    info.GetStatus().String(),
		TaskQueue: info.GetTaskQueue(),
	}
	if ref := info.GetExecution(); ref != nil {
		out.WorkflowID = ref.GetWorkflowId()
		out.RunID = ref.GetRunId()
	}
	if stamp := info.GetStartTime(); stamp != nil {
		out.StartTime = stamp.AsTime().UTC().Format(time.RFC3339)
	}
	if stamp := info.GetCloseTime(); stamp != nil {
		out.CloseTime = stamp.AsTime().UTC().Format(time.RFC3339)
	}
	return out
}

func failure(err error) *mcp.CallToolResult {
	return failureText(err.Error())
}

func failureText(message string) *mcp.CallToolResult {
	return &mcp.CallToolResult{
		IsError: true,
		Content: []mcp.Content{&mcp.TextContent{Text: message}},
	}
}
