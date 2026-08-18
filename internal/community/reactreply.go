package community

import (
	"context"
	"fmt"
	"log/slog"
	"regexp"
	"sort"
	"strings"

	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/trace"
)

// Some answers are a mark on the member's own message rather than a message of
// their own. See docs/sirens-echo-progress.md.

// reactInvocation is what the model writes to answer with a mark. The same
// shape a phrase takes, so one alphabet covers both.
var reactInvocation = regexp.MustCompile(`\{\{react:([^}]*)\}\}`)

// replyReactions is the closed set a reply may invoke, keyed rather than
// spelled. Disjoint from the harness marks. See docs/sirens-echo-phrases.md.
var replyReactions = map[string]string{
	"agree":       "\U0001F44D", // 👍
	"disagree":    "\U0001F44E", // 👎
	"acknowledge": "\u2705",     // ✅
}

// reactKeys names every key, for the prompt that tells the model what it may
// invoke. Sorted, so the prompt is the same string on every boot.
func reactKeys() []string {
	keys := make([]string, 0, len(replyReactions))
	for key := range replyReactions {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	return keys
}

// reactInvoked reports whether a reply carries an invocation at all, so an
// ordinary reply is left untouched.
func reactInvoked(reply string) bool { return reactInvocation.MatchString(reply) }

// reactTerminal reports whether one invocation is the whole reply. Exactly one,
// because two marks is not a mark and a mark beside prose is prose.
func reactTerminal(reply string) bool {
	if len(reactInvocation.FindAllStringIndex(reply, -1)) != 1 {
		return false
	}
	return strings.TrimSpace(reactInvocation.ReplaceAllString(reply, "")) == ""
}

// resolveReaction turns an invocation into the glyph it names. An unknown key
// is an error rather than a marker a member reads.
func (a *Agent) resolveReaction(ctx context.Context, reply string) (string, error) {
	if !reactTerminal(reply) {
		return "", fmt.Errorf("reply invokes a reaction alongside other text")
	}
	key := strings.TrimSpace(reactInvocation.FindStringSubmatch(reply)[1])
	glyph, known := replyReactions[key]
	if !known {
		return "", fmt.Errorf("reply invokes unknown reaction %q", key)
	}
	// The key set is closed and service-authored, so it is safe as a label where
	// a member-supplied value would not be. See docs/sirens-echo-admission.md.
	a.telemetry.Info(ctx, "response.reaction.invoked", slog.String("reaction.key", key))
	trace.SpanFromContext(ctx).SetAttributes(attribute.String("response.reaction", key))
	return glyph, nil
}

// finishByReacting ends a turn whose whole answer is a mark. The mark is the
// delivery here, so a failed one falls back to words rather than being dropped.
func (a *Agent) finishByReacting(
	ctx context.Context,
	turn turnIO,
	progress *turnProgress,
	target reactor,
	glyph string,
) error {
	// A line that just went up should be readable before the answer lands, the
	// same courtesy a sent reply gets. See docs/sirens-echo-progress.md.
	a.settleWithSpan(ctx, progress.settleDelay(), progress.Settle)
	if err := turnReactor(ctx, target).React(ctx, glyph); err != nil {
		a.telemetry.Info(
			ctx,
			"response.reaction.undelivered",
			slog.String("reaction", glyph),
			slog.String("error", err.Error()),
		)
		// Likeliest a missing ADD_REACTIONS permission, which must not cost the
		// member the answer. The glyph it stood for is sent as the reply.
		if err := a.deliverOrReport(ctx, turn, glyph, nothingWithheld); err != nil {
			return err
		}
		a.clearTurnMarks(ctx)
		a.beats.reply()
		return nil
	}
	a.telemetry.Info(ctx, "turn.reply.reaction", slog.String("reaction", glyph))
	// The mark is the outcome, so nothing is left to describe work in flight.
	a.clearTurnMarks(ctx)
	a.beats.reply()
	return nil
}

// withReactionPolicy names the keys a reply may invoke. Appended for every
// deployment, because the set is compiled in rather than configured.
func withReactionPolicy(prompt string) string {
	return prompt + "\n" + fmt.Sprintf(
		`Some answers are a reaction on the member's own message rather than a reply.
Invoke one by writing {{react:key}} and nothing else, because an invocation is
the whole reply and a reaction beside other text is refused. Use one when
acknowledging, agreeing, or declining is the entire answer and words would add
nothing, and answer normally otherwise. Available keys: %s.`,
		strings.Join(reactKeys(), ", "),
	) + "\n"
}
