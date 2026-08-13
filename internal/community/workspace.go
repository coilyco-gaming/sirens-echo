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
)

// A workspace is one job's private directory. See docs/sirens-echo-execution.md.

const (
	// maxCommandOutputBytes bounds what one command can return, so a runaway
	// build cannot be read into memory unbounded.
	maxCommandOutputBytes = 64 << 10
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
	runCtx, cancel := context.WithTimeout(ctx, r.timeout())
	defer cancel()
	command := exec.CommandContext(runCtx, r.binary(), append([]string{name}, args...)...)
	command.Dir = workspace.Root
	// An explicit environment rather than the process's own, so a command never
	// inherits a token this process holds.
	command.Env = append([]string{}, r.Environment...)
	raw, runErr := command.CombinedOutput()
	result := CommandResult{Output: string(raw)}
	if len(raw) > maxCommandOutputBytes {
		result.Output = string(raw[:maxCommandOutputBytes])
		result.Truncated = true
	}
	if runErr != nil {
		var exitErr *exec.ExitError
		if asExitError(runErr, &exitErr) {
			result.ExitCode = exitErr.ExitCode()
			return result, fmt.Errorf("command %s exited %d", name, result.ExitCode)
		}
		return result, fmt.Errorf("command %s did not run", name)
	}
	return result, nil
}

// asExitError narrows a run failure to a process that started and exited.
func asExitError(err error, target **exec.ExitError) bool {
	return errors.As(err, target)
}
