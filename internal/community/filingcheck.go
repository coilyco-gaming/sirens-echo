package community

import (
	"context"
	"fmt"
	"log/slog"
	"strings"
)

// Two stages a member-originated ticket passes before it reaches the tracker.
// See docs/sirens-echo-issues.md.

// filingStage names which check refused. It reaches a span and a log, so the
// set is closed and never carries model output.
type filingStage string

const (
	filingStageValidity   filingStage = "validity"
	filingStageClassifier filingStage = "classifier"
	// filingCheckRequestID audits these calls as the harness's own. A requester
	// id here would put a member identifier in the proxy's audit metadata.
	filingCheckRequestID = "filing-check"
)

// The closed vocabularies. A classifier answering off its list is a classifier
// failing rather than refusing, which is the contentgate rule one file over.
const (
	filingVerdictWork        = "work"
	filingVerdictPlaceholder = "placeholder"
	filingVerdictUnclear     = "unclear"
	filingVerdictInScope     = "in-scope"
	filingVerdictOutOfScope  = "out-of-scope"
)

// filingValidityPrompt asks whether there is work in the request. It is not
// asking whether the request is hostile. See docs/sirens-echo-issues.md.
const filingValidityPrompt = `Decide whether a proposed tracker issue contains actionable work.
Answer with exactly one word and nothing else.

work - it names something specific that someone could do or fix.
placeholder - it is a note to a future reader, a reminder, or a restatement of
an intention, with nothing to act on.
unclear - it is on topic but too vague for anyone to act on without asking.

Politeness, tone, and good faith are not the question. A courteous, well-formed
request with nothing to act on is a placeholder.`

// filingScopePrompt is the second stage, on the same text and a different axis.
const filingScopePrompt = `Decide whether a proposed tracker issue belongs to this service:
the Discord agent, its harness, its tools, and its deployment.
Answer with exactly one word and nothing else.

in-scope - it concerns this service or something it does.
out-of-scope - it concerns a game, a community matter, another product, or a
question that wanted an answer rather than a ticket.`

// filingRefusal is the harness-authored sentence a refused filing returns. The
// model reads it and tells the member, so it says what would fix it.
var filingRefusal = map[string]string{
	filingVerdictPlaceholder: "that request has nothing to act on yet. Say what should change " +
		"and what is wrong with it now, and it can be filed.",
	filingVerdictUnclear: "that request is too vague to file. Name the specific behaviour " +
		"and what you expected instead.",
	filingVerdictOutOfScope: "that is not about this service, so it does not belong on this " +
		"tracker. Ask it here and it can be answered directly.",
}

// checkMemberFiling runs the stages for a member. The principal is exempt,
// because an admin filing is not the failure mode this exists for.
func (a *Agent) checkMemberFiling(ctx context.Context, title, body string) error {
	if a.cfg.Principal.Configured() &&
		RequesterFrom(ctx) == a.cfg.Principal.UserID {
		return nil
	}
	return a.filingCheck(ctx, filingCheckRequestID, title, body)
}

// filingCheck runs both stages over the proposed issue. A nil error files it.
func (a *Agent) filingCheck(ctx context.Context, requestID, title, body string) error {
	if a.completions == nil {
		return nil
	}
	proposed := strings.TrimSpace(title + "\n\n" + body)
	if proposed == "" {
		return nil
	}
	for _, stage := range []struct {
		name    filingStage
		prompt  string
		refuses map[string]bool
		allowed map[string]bool
	}{
		{
			name:    filingStageValidity,
			prompt:  filingValidityPrompt,
			refuses: map[string]bool{filingVerdictPlaceholder: true, filingVerdictUnclear: true},
			allowed: map[string]bool{
				filingVerdictWork: true, filingVerdictPlaceholder: true, filingVerdictUnclear: true,
			},
		},
		{
			name:    filingStageClassifier,
			prompt:  filingScopePrompt,
			refuses: map[string]bool{filingVerdictOutOfScope: true},
			allowed: map[string]bool{filingVerdictInScope: true, filingVerdictOutOfScope: true},
		},
	} {
		verdict, err := a.filingVerdict(ctx, requestID, stage.prompt, proposed, stage.allowed)
		if err != nil {
			// A classifier that fails is not a denial, matching the content
			// gate. See docs/sirens-echo-content-gate.md.
			a.telemetry.Error(ctx, "issue.filing.check.failed",
				slog.String("stage", string(stage.name)),
				slog.String("error", err.Error()))
			return nil
		}
		if stage.refuses[verdict] {
			return &filingRefused{Stage: stage.name, Verdict: verdict}
		}
	}
	return nil
}

// filingVerdict asks one stage and refuses an answer off its list, so an
// invented word is a failure rather than a silent pass.
func (a *Agent) filingVerdict(
	ctx context.Context,
	requestID, prompt, proposed string,
	allowed map[string]bool,
) (string, error) {
	result, err := a.completions.Complete(
		ctx, TurnPrompt{System: prompt, Message: proposed}, requestID)
	if err != nil {
		return "", fmt.Errorf("filing check: %w", err)
	}
	answer := strings.ToLower(strings.Trim(strings.TrimSpace(result.Content), ".`\"' \n"))
	if !allowed[answer] {
		return "", fmt.Errorf("filing check answered off its list")
	}
	return answer, nil
}

// filingRefused is a refused filing. It carries the stage and the verdict and
// no model text, so a span can take both.
type filingRefused struct {
	Stage   filingStage
	Verdict string
}

func (r *filingRefused) Error() string {
	if sentence, known := filingRefusal[r.Verdict]; known {
		return sentence
	}
	return "that request cannot be filed as it stands"
}
