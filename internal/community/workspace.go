package community

import (
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"go.opentelemetry.io/otel/attribute"
)

// A workspace is one job's private directory. See docs/sirens-echo-execution.md.

const (
	// workspacePermissions keeps a job's tree readable only by this process.
	workspacePermissions = 0o700
)

// Workspace is a per-job directory, removed when the job reaches any terminal
// state including cancellation and crash.
type Workspace struct {
	Root  string
	JobID string
}

// OpenWorkspace creates the job's directory. Two jobs never share one, because
// the path is derived from the id and the id is unique.
func OpenWorkspace(root, jobID string) (*Workspace, error) {
	if strings.TrimSpace(root) == "" {
		return nil, fmt.Errorf("workspace root is required")
	}
	if !validJobID(jobID) {
		return nil, fmt.Errorf("workspace needs a valid job id")
	}
	path := filepath.Join(root, jobID)
	// A leftover from a previous attempt is removed rather than reused, so an
	// attempt always starts from a known state.
	if err := os.RemoveAll(path); err != nil {
		return nil, fmt.Errorf("clear workspace: %w", err)
	}
	if err := os.MkdirAll(path, workspacePermissions); err != nil {
		return nil, fmt.Errorf("create workspace: %w", err)
	}
	return &Workspace{Root: path, JobID: jobID}, nil
}

// Remove deletes the workspace. It is safe to call more than once, because a
// terminal state can be reached by more than one path.
func (w *Workspace) Remove() error {
	if w == nil || w.Root == "" {
		return nil
	}
	return os.RemoveAll(w.Root)
}

// CommandResult is one bounded execution.
type CommandResult struct {
	Output    string
	Truncated bool
	ExitCode  int
}

// CommandRunner executes one bounded command in a workspace. It exists so the
// executor can be tested without a real ward on the machine.
type CommandRunner interface {
	Run(ctx context.Context, workspace *Workspace, name string, args ...string) (CommandResult, error)
}

// WardCommandRunner routes execution through ward, the governed execution
// layer. There is deliberately no second path that bypasses it.
type WardCommandRunner struct {
	// Binary is the ward entry point. Empty uses ward on PATH.
	Binary string
	// Timeout bounds one command. Zero uses defaultCommandTimeout.
	Timeout time.Duration
	// Environment is the exact environment a command gets. Nil means empty,
	// so a command inherits nothing this process holds.
	Environment []string
	// Telemetry records that a command ran. Nil is the no-op recorder.
	Telemetry *Telemetry
}

func (r WardCommandRunner) binary() string {
	if strings.TrimSpace(r.Binary) != "" {
		return r.Binary
	}
	return "ward"
}

func (r WardCommandRunner) timeout() time.Duration {
	if r.Timeout > 0 {
		return r.Timeout
	}
	return defaultCommandTimeout
}

// Run executes inside the workspace with a bounded clock and bounded output.
func (r WardCommandRunner) Run(
	ctx context.Context,
	workspace *Workspace,
	name string,
	args ...string,
) (CommandResult, error) {
	if workspace == nil || workspace.Root == "" {
		return CommandResult{}, fmt.Errorf("a command needs a workspace")
	}
	telemetry := telemetryOrNoop(r.Telemetry)
	// The verb and the job, never the arguments: a clone argument is a URL.
	// See docs/sirens-echo-telemetry.md.
	runCtx, span := telemetry.StartSpan(
		ctx,
		"job.command",
		attribute.String("command.verb", name),
		attribute.String("job.id", workspace.JobID),
	)
	defer span.End()
	runCtx, cancel := context.WithTimeout(runCtx, r.timeout())
	defer cancel()
	command := exec.CommandContext(runCtx, r.binary(), append([]string{name}, args...)...)
	command.Dir = workspace.Root
	// Killing the command does not close a pipe a grandchild still holds, so the
	// deadline bounded nothing without this. See docs/sirens-echo-execution.md.
	command.WaitDelay = commandKillGrace
	// An explicit environment rather than the process's own, so a command never
	// inherits a token this process holds.
	command.Env = append([]string{}, r.Environment...)
	startedAt := time.Now()
	raw, runErr := command.CombinedOutput()
	span.SetAttributes(
		attribute.Int64("command.millis", time.Since(startedAt).Milliseconds()),
		attribute.Bool("command.truncated", len(raw) > maxCommandOutputBytes),
	)
	result := CommandResult{Output: string(raw)}
	if len(raw) > maxCommandOutputBytes {
		result.Output = string(raw[:maxCommandOutputBytes])
		result.Truncated = true
	}
	if runErr != nil {
		var exitErr *exec.ExitError
		telemetry.MarkSpanError(span, exceptionCommandFailed)
		if asExitError(runErr, &exitErr) {
			result.ExitCode = exitErr.ExitCode()
			// The code on the span, not the metric: 0 to 255 is cardinality a
			// closed outcome label does not want.
			span.SetAttributes(attribute.Int("command.exit_code", result.ExitCode))
			telemetry.RecordCommand(runCtx, name, "exited")
			return result, fmt.Errorf("command %s exited %d", name, result.ExitCode)
		}
		telemetry.RecordCommand(runCtx, name, "did_not_run")
		return result, fmt.Errorf("command %s did not run", name)
	}
	telemetry.RecordCommand(runCtx, name, "ok")
	return result, nil
}

// asExitError narrows a run failure to a process that started and exited.
func asExitError(err error, target **exec.ExitError) bool {
	return errors.As(err, target)
}
