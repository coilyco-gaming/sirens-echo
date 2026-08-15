package community

import (
	"context"
	"errors"
	"log/slog"

	"github.com/bwmarrin/discordgo"
)

// A delivery failure used to record that it happened and discard why. See
// docs/sirens-echo-delivery-failures.md.

// undeliveredReply marks a turn error as the reply send itself failing. See
// docs/sirens-echo-turn-verdict.md and #292.
type undeliveredReply struct{ err error }

func (u *undeliveredReply) Error() string { return u.err.Error() }

func (u *undeliveredReply) Unwrap() error { return u.err }

// turnFailureAttrs classifies a turn error, which is a Discord verdict only
// when the reply send is what failed.
func turnFailureAttrs(err error) []slog.Attr {
	var undelivered *undeliveredReply
	if !errors.As(err, &undelivered) {
		// A distinct state rather than a plausible one: the turn ended for a
		// reason Discord was never asked about.
		return []slog.Attr{slog.String("discord_failure", "not_attempted")}
	}
	return discordFailureAttrs(undelivered.Unwrap())
}

// discordFailureAttrs describes a failed send in closed-set terms. Status and
// code separate rate limiting, length, permissions, and a dropped gateway.
func discordFailureAttrs(err error) []slog.Attr {
	if err == nil {
		return nil
	}
	// Ahead of the context check, because the turn reports a join of the send
	// and its notice, and Discord answering outranks our budget. See #292.
	var rest *discordgo.RESTError
	if !errors.As(err, &rest) {
		// A budget this service chose ended this and nothing was wrong with
		// Discord. See sirens-echo#648.
		if errors.Is(err, context.DeadlineExceeded) || errors.Is(err, context.Canceled) {
			return []slog.Attr{slog.String("discord_failure", "abandoned")}
		}
		// No HTTP exchange happened, which is itself the classification: the
		// gateway or the transport failed before Discord answered.
		return []slog.Attr{slog.String("discord_failure", "no_response")}
	}
	attrs := []slog.Attr{slog.String("discord_failure", "rest_error")}
	if rest.Response != nil {
		attrs = append(attrs, slog.Int("discord_status", rest.Response.StatusCode))
	}
	// Discord's own code is the field that separates two failures sharing one
	// status, such as a missing permission from a blocked recipient.
	if rest.Message != nil {
		attrs = append(attrs, slog.Int("discord_code", rest.Message.Code))
	}
	return attrs
}
