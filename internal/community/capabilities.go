package community

import (
	"context"
	"log/slog"
)

// A capability that was never switched on is indistinguishable from inside the
// process from one that was never built. See docs/sirens-echo.md.

// These name the stores, so a reader learns which one holds jobs. The durable
// two stay distinct: file survives a restart, postgres survives the pod.
const (
	jobStorePostgres = "postgres"
	jobStoreDurable  = "file"
	jobStoreMemory   = "memory"
)

// logCapabilities states what this process can and cannot do, once, at boot.
// Presence only. A path or a channel id here would grow into an identifier.
func (a *Agent) logCapabilities(ctx context.Context) {
	a.telemetry.Info(
		ctx,
		"capabilities",
		slog.String("profile", a.cfg.Definition.Identity),
		slog.Bool("discord", a.cfg.DiscordEnabled),
		slog.Bool("discord_direct_messages", a.cfg.DiscordDMEnabled),
		slog.Bool("discord_commands", a.cfg.DiscordCommandsEnabled),
		slog.Bool("content_gate", len(a.taxonomy.Classes) > 0),
		slog.Bool("phrase_registry", a.phrases.Configured()),
		slog.String("job_store", a.jobStoreKind()),
		slog.Bool("jobs", a.jobs != nil),
		slog.Bool("scratchpad", a.cfg.ScratchDir != ""),
		slog.Bool("fetch", len(a.cfg.FetchHosts) > 0),
		slog.Bool("issue_tracker", a.cfg.Definition.IssueTracker != ""),
		slog.Int("mcp_servers", a.rosterSize()),
		// Names, never values: a knob's value is already in the reference and
		// the interesting part is which ones this deployment moved.
		slog.Any("tuning_applied", a.cfg.TuningApplied),
		// A name this service could not parse kept its default. Silence there
		// reads as a working override. See docs/sirens-echo-tuning.md.
		slog.Any("tuning_rejected", a.cfg.TuningRejected),
	)
}

// jobStoreKind reports which store is holding jobs, because the durable one and
// the one that drops everything on a roll are the same field to a reader.
func (a *Agent) jobStoreKind() string {
	switch {
	case a.cfg.JobStoreDSN != "":
		return jobStorePostgres
	case a.cfg.JobStoreDir != "":
		return jobStoreDurable
	default:
		return jobStoreMemory
	}
}

func (a *Agent) rosterSize() int {
	if a.tools == nil {
		return 0
	}
	return len(a.tools.Servers)
}
