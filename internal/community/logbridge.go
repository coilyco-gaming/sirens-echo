package community

import (
	"context"
	"errors"
	"log/slog"
)

// Logs go to stdout and over OTLP at once, so one line reaches both a kubectl
// reader and SigNoz. See docs/sirens-echo-log-export.md.

// multiHandler writes one record to every handler. slog offers no fan-out, and
// swapping stdout for OTLP would cost kubectl logs during an incident.
type multiHandler struct {
	handlers []slog.Handler
}

// Enabled reports true when any destination wants the level, so a quiet one
// cannot silence a talkative one.
func (h multiHandler) Enabled(ctx context.Context, level slog.Level) bool {
	for _, handler := range h.handlers {
		if handler.Enabled(ctx, level) {
			return true
		}
	}
	return false
}

// Handle writes to every enabled destination and joins their errors, so a
// failing exporter does not stop the line reaching stdout.
func (h multiHandler) Handle(ctx context.Context, record slog.Record) error {
	var errs []error
	for _, handler := range h.handlers {
		if !handler.Enabled(ctx, record.Level) {
			continue
		}
		// Cloned because a handler may retain or modify what it is given, and
		// the next one must see the record as it arrived.
		errs = append(errs, handler.Handle(ctx, record.Clone()))
	}
	return errors.Join(errs...)
}

func (h multiHandler) WithAttrs(attrs []slog.Attr) slog.Handler {
	return multiHandler{handlers: mapHandlers(h.handlers, func(inner slog.Handler) slog.Handler {
		return inner.WithAttrs(attrs)
	})}
}

func (h multiHandler) WithGroup(name string) slog.Handler {
	return multiHandler{handlers: mapHandlers(h.handlers, func(inner slog.Handler) slog.Handler {
		return inner.WithGroup(name)
	})}
}

// mapHandlers derives a new set without mutating the receiver's, so a logger
// built from one of these does not reshape the logger it came from.
func mapHandlers(handlers []slog.Handler, derive func(slog.Handler) slog.Handler) []slog.Handler {
	derived := make([]slog.Handler, len(handlers))
	for index, handler := range handlers {
		derived[index] = derive(handler)
	}
	return derived
}
