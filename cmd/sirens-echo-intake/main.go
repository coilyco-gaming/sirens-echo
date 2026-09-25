// Command sirens-echo-intake holds a Discord gateway session and writes every
// event it receives to the Postgres queue the worker answers from. Several run
// at once. See docs/sirens-echo-jobs.md.
package main

import (
	"context"
	"log/slog"
	"os"
	"os/signal"
	"syscall"
	"time"

	"forgejo.coilysiren.me/coilyco-gaming/sirens-echo/internal/community"
)

func main() {
	startup := slog.New(slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelInfo}))
	cfg, err := community.LoadIntakeConfig()
	if err != nil {
		startup.Error("startup.config.failed", slog.String("error", err.Error()))
		os.Exit(1)
	}
	telemetry, err := community.NewTelemetry(context.Background(), cfg)
	if err != nil {
		startup.Error("startup.telemetry.failed", slog.String("error", err.Error()))
		os.Exit(1)
	}
	defer func() {
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		_ = telemetry.Close(shutdownCtx)
	}()
	intake, err := community.NewIntake(cfg, telemetry)
	if err != nil {
		telemetry.Error(context.Background(), "startup.intake.failed", slog.String("error", err.Error()))
		os.Exit(1)
	}
	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer cancel()
	if err := intake.Run(ctx); err != nil {
		telemetry.Error(ctx, "run.failed", slog.String("error", err.Error()))
		os.Exit(1)
	}
}
