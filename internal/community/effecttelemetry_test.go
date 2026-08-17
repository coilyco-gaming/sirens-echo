package community

import (
	"context"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// A command execution and an attachment fetch are effects this service caused,
// and neither was recorded anywhere. See sirens-echo#890.

// scriptRunner points the runner at a shell rather than ward, so a test can
// choose the exit status without one installed.
func scriptRunner(t *testing.T, telemetry *Telemetry) (WardCommandRunner, *Workspace) {
	t.Helper()
	workspace, err := OpenWorkspace(t.TempDir(), "job-abcdef01")
	if err != nil {
		t.Fatalf("OpenWorkspace: %v", err)
	}
	t.Cleanup(func() { _ = workspace.Remove() })
	return WardCommandRunner{Binary: "sh", Telemetry: telemetry}, workspace
}

// A command that ran is recorded, and the record names the verb rather than the
// arguments, because an argument carries a clone URL.
func TestASucceedingCommandIsRecorded(t *testing.T) {
	t.Parallel()
	telemetry := telemetryOrNoop(nil)
	runner, workspace := scriptRunner(t, telemetry)

	result, err := runner.Run(context.Background(), workspace, "-c", "exit 0")
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	if result.ExitCode != 0 {
		t.Errorf("exit code = %d", result.ExitCode)
	}
}

// A non-zero exit is a distinct outcome from a command that never started, and
// collapsing them would lose which one an operator is looking at.
func TestAFailingCommandAndAMissingOneAreDifferentOutcomes(t *testing.T) {
	t.Parallel()
	telemetry := telemetryOrNoop(nil)
	runner, workspace := scriptRunner(t, telemetry)

	_, err := runner.Run(context.Background(), workspace, "-c", "exit 3")
	if err == nil || !strings.Contains(err.Error(), "exited 3") {
		t.Fatalf("a non-zero exit reported %v", err)
	}

	missing := WardCommandRunner{
		Binary:    filepath.Join(t.TempDir(), "not-a-binary"),
		Telemetry: telemetry,
	}
	_, err = missing.Run(context.Background(), workspace, "anything")
	if err == nil || !strings.Contains(err.Error(), "did not run") {
		t.Fatalf("a missing binary reported %v", err)
	}
}

// The runner must keep working without telemetry, because a nil recorder is the
// shape every existing caller and test uses.
func TestACommandRunsWithoutTelemetry(t *testing.T) {
	t.Parallel()
	workspace, err := OpenWorkspace(t.TempDir(), "job-abcdef02")
	if err != nil {
		t.Fatalf("OpenWorkspace: %v", err)
	}
	t.Cleanup(func() { _ = workspace.Remove() })
	if _, err := (WardCommandRunner{Binary: "sh"}).Run(
		context.Background(), workspace, "-c", "exit 0",
	); err != nil {
		t.Fatalf("Run without telemetry: %v", err)
	}
}

// The workspace still bounds the command, which the added span must not have
// moved off the context.
func TestTheCommandTimeoutStillApplies(t *testing.T) {
	t.Parallel()
	workspace, err := OpenWorkspace(t.TempDir(), "job-abcdef03")
	if err != nil {
		t.Fatalf("OpenWorkspace: %v", err)
	}
	t.Cleanup(func() { _ = workspace.Remove() })
	runner := WardCommandRunner{Binary: "sh", Timeout: 50 * time.Millisecond}

	started := time.Now()
	if _, err := runner.Run(context.Background(), workspace, "-c", "sleep 10"); err == nil {
		t.Fatal("a command outran its own timeout")
	}
	if elapsed := time.Since(started); elapsed > 5*time.Second {
		t.Errorf("the timeout took %s to fire", elapsed)
	}
}

// Every ingest arm records, including the refusals that used to return
// silently. A refused upload and no upload at all read identically otherwise.
func TestEveryAttachmentOutcomeIsRecorded(t *testing.T) {
	t.Parallel()
	telemetry := telemetryOrNoop(nil)

	// A host outside the CDN allowlist is refused before any fetch.
	stored := ingestAttachments(
		WithAttachments(context.Background(), []AttachmentSource{
			{URL: "https://example.invalid/upload.txt"},
		}),
		&recordingReservedSession{}, nil, telemetry,
	)
	if len(stored) != 0 {
		t.Errorf("an unpermitted host was stored: %v", stored)
	}
}

// A fetch that fails is a span error and a counted outcome rather than a silent
// continue, which is what made an ingest failure invisible.
func TestAFailedAttachmentFetchIsRecorded(t *testing.T) {
	t.Parallel()
	server := httptest.NewServer(http.HandlerFunc(
		func(writer http.ResponseWriter, _ *http.Request) {
			writer.WriteHeader(http.StatusInternalServerError)
		}))
	defer server.Close()

	_, err := fetchAttachment(
		context.Background(), server.Client(), server.URL, telemetryOrNoop(nil),
	)
	if err == nil {
		t.Fatal("a 500 reported success")
	}
}

// The acceptance that keeps the two halves of sirens-echo#890 apart: adding
// telemetry must not start exporting these to a third-party SaaS.
func TestNeitherEffectReachesTheTemporalMirror(t *testing.T) {
	t.Parallel()
	telemetry := telemetryOrNoop(nil)
	mirror := newRecordingMirror()
	telemetry.AttachToolMirror(mirror)
	defer telemetry.CloseToolMirror()

	runner, workspace := scriptRunner(t, telemetry)
	if _, err := runner.Run(context.Background(), workspace, "-c", "exit 0"); err != nil {
		t.Fatalf("Run: %v", err)
	}
	telemetry.RecordCommand(context.Background(), "exec", "ok")
	telemetry.RecordAttachment(context.Background(), "stored")

	// A tool call does mirror, so this proves the mirror is live rather than
	// that the test simply never delivered anything.
	telemetry.RecordToolCall(context.Background(), "eco", "get_market", "ok", time.Millisecond)
	mirror.waitFor(t, 1)

	for _, record := range mirror.seen() {
		if record.Server != "eco" {
			t.Errorf("a non-tool effect reached the mirror: %#v", record)
		}
	}
	if got := len(mirror.seen()); got != 1 {
		t.Errorf("mirrored %d records, want only the tool call", got)
	}
}

// recordingReservedSession is a scratch session that accepts reserved writes.
type recordingReservedSession struct {
	written map[string]string
}

func (s *recordingReservedSession) Tools() []ToolDefinition        { return nil }
func (s *recordingReservedSession) Grounding() []GroundingDocument { return nil }
func (s *recordingReservedSession) Guidance() []ServerGuidance     { return nil }
func (s *recordingReservedSession) Unavailable() []string          { return nil }
func (s *recordingReservedSession) Close() error                   { return nil }
func (s *recordingReservedSession) Call(
	context.Context, string, map[string]any,
) (ToolResult, error) {
	return ToolResult{}, nil
}

func (s *recordingReservedSession) WriteReserved(path, body string) (ToolResult, error) {
	if s.written == nil {
		s.written = map[string]string{}
	}
	s.written[path] = body
	return ToolResult{Text: "stored"}, nil
}
