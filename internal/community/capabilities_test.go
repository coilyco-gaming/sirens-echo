package community

import (
	"bytes"
	"context"
	"log/slog"
	"strings"
	"testing"

	metricnoop "go.opentelemetry.io/otel/metric/noop"
	tracenoop "go.opentelemetry.io/otel/trace/noop"
)

// The state this reports is the one that costs two greps across two
// repositories to establish today. See sirens-echo#539.

// capabilityLine captures what the line actually emits. telemetryOrNoop writes
// to io.Discard, so a test built on it asserts against an empty string.
func capabilityLine(t *testing.T, agent *Agent) string {
	t.Helper()
	var recorded bytes.Buffer
	telemetry, err := newTelemetry(
		slog.New(slog.NewJSONHandler(&recorded, nil)),
		tracenoop.NewTracerProvider(),
		metricnoop.NewMeterProvider(),
	)
	if err != nil {
		t.Fatalf("build telemetry: %v", err)
	}
	agent.telemetry = telemetry
	agent.logCapabilities(context.Background())

	line := recorded.String()
	if !strings.Contains(line, "capabilities") {
		t.Fatalf("nothing was captured, so this test asserts nothing: %q", line)
	}
	return line
}

// A process with nothing switched on says so, rather than saying nothing.
func TestAnInertProcessReportsEveryCapabilityOff(t *testing.T) {
	t.Parallel()
	agent := &Agent{telemetry: telemetryOrNoop(nil)}
	// Reaching here without a panic on a hand-built Agent is half the point:
	// the line must not depend on a fully wired runtime.
	agent.logCapabilities(context.Background())

	if got := agent.jobStoreKind(); got != jobStoreMemory {
		t.Errorf("job store = %q, want %q for an unset directory", got, jobStoreMemory)
	}
	if got := agent.rosterSize(); got != 0 {
		t.Errorf("roster size = %d, want 0 with no provider", got)
	}
}

// The distinction the issue was filed about: a durable store and the one that
// drops every in-flight job on a roll are the same field to a reader.
func TestTheJobStoreKindIsReported(t *testing.T) {
	t.Parallel()
	durable := &Agent{}
	durable.cfg.JobStoreDir = "/var/lib/sirens-echo/jobs"
	if got := durable.jobStoreKind(); got != jobStoreDurable {
		t.Errorf("job store = %q, want %q", got, jobStoreDurable)
	}
}

// Presence only. A path or a channel id in this line grows into an identifier,
// which is a thing this service has a guard against elsewhere.
func TestTheCapabilityLineCarriesNoValues(t *testing.T) {
	t.Parallel()
	agent := &Agent{telemetry: telemetryOrNoop(nil)}
	agent.cfg.ScratchDir = "/var/lib/sirens-echo/scratch"
	agent.cfg.FetchHosts = []string{"wiki.example"}
	agent.cfg.Definition.IssueTracker = "sirens-echo-forgejo"

	line := capabilityLine(t, agent)
	for _, secret := range []string{
		"/var/lib/sirens-echo/scratch",
		"wiki.example",
	} {
		if strings.Contains(line, secret) {
			t.Errorf("the capability line carries the value %q, not just its presence", secret)
		}
	}
}

// The roster size is a count rather than a list, for the same reason.
func TestTheRosterIsCountedNotNamed(t *testing.T) {
	t.Parallel()
	agent := &Agent{
		telemetry: telemetryOrNoop(nil),
		tools: &MCPProvider{Servers: []MCPServerDefinition{
			{Name: "eco", URL: "http://127.0.0.1:1/mcp"},
			{Name: "forgejo", URL: "http://127.0.0.1:1/mcp"},
		}},
	}
	if got := agent.rosterSize(); got != 2 {
		t.Fatalf("roster size = %d, want 2", got)
	}
	if line := capabilityLine(t, agent); strings.Contains(line, "127.0.0.1") {
		t.Errorf("the capability line carries a server URL: %s", line)
	}
}
