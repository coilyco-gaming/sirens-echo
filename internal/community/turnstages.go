package community

import "errors"

// A rejected reply was attributable to the model rather than to the check that
// refused it. See docs/sirens-echo-turn-stages.md.

// The reply checks, named so a refusal says which one refused. These are metric
// and attribute values, so the set is closed and the spelling is stable.
const (
	replyCheckNone           = "none"
	replyCheckParse          = "parse"
	replyCheckToolCallMarkup = "tool_call_markup"
	replyCheckGrounding      = "grounding"
	replyCheckSelfAttributed = "self_attributed_claim"
	replyCheckIdentifiers    = "identifier_disclosure"
	replyCheckIdentityClaim  = "identity_claim"
	replyCheckResponseStyle  = "response_style"
)

// qualityChecks are the rules about how an answer reads. A quality refusal is
// recorded and the reply still ships. See docs/sirens-echo-delivery.md.
var qualityChecks = map[string]bool{
	replyCheckResponseStyle:  true,
	replyCheckToolCallMarkup: true,
}

// checkGates reports a refusal that must not reach a member. Unlisted gates, so
// a new rule fails closed rather than shipping. See sirens-echo#651.
func checkGates(check string) bool { return !qualityChecks[check] }

// The grounding rules. The family covers four independent checks, so its slug
// narrowed nothing. See docs/sirens-echo-grounding.md.
const (
	replyCheckInventedChannel = replyCheckGrounding + ".invented_channel"
	replyCheckClaimedAction   = replyCheckGrounding + ".claimed_action"
	replyCheckTrackerAction   = replyCheckGrounding + ".tracker_action"
	replyCheckContinuingWork  = replyCheckGrounding + ".continuing_work"
)

// checkRefusal is a refusal that names the rule behind it. Ordinary errors stay
// ordinary errors: a validator with one rule has nothing to add to its family.
type checkRefusal struct {
	rule   string
	reason string
}

func (r checkRefusal) Error() string { return r.reason }

// refusedBy resolves the attribute value for a refusal, falling back to the
// family when the validator declares no rule of its own.
func refusedBy(family string, err error) string {
	var refusal checkRefusal
	if errors.As(err, &refusal) && refusal.rule != "" {
		return refusal.rule
	}
	return family
}

// repairableReplyChecks is runReplyChecks as the completion layer's repair loop
// consumes it. The error alone, because a refusal already carries its rule.
func (a *Agent) repairableReplyChecks(
	reply string,
	prompt TurnPrompt,
	executed []ExecutedTool,
) error {
	_, _, err := a.runReplyChecks(reply, prompt, CompletionResult{ToolCalls: executed})
	return err
}

// runReplyChecks runs the checks in order and reports which one refused. The
// order is the contract, so this is a slice rather than a chain of conditions.
func (a *Agent) runReplyChecks(
	reply string,
	prompt TurnPrompt,
	result CompletionResult,
) (string, string, error) {
	checks := []struct {
		name string
		run  func() error
	}{
		// Nothing else between the model and the member sees this, and a member
		// reads it verbatim. See docs/sirens-echo-config.md.
		{replyCheckToolCallMarkup, func() error { return ValidateNoToolCallMarkup(reply) }},
		{replyCheckGrounding, func() error {
			return ValidateGrounding(reply, prompt.Supplied(), result.ToolCalls...)
		}},
		{replyCheckSelfAttributed, func() error {
			return ValidateSelfAttributedClaim(
				reply, a.cfg.Definition.Identity,
				a.cfg.Definition.SelfAliases, result.ToolCalls...)
		}},
		// Output values are enumerable where input framings are not, so this is
		// the check that does not depend on anticipating the framing.
		{replyCheckIdentifiers, func() error { return a.identifiers.Validate(reply) }},
		// Bound for every style. Not being mistaken for a human is a safety
		// property, not a voice preference. See docs/sirens-echo-prompt.md.
		{replyCheckIdentityClaim, func() error {
			return ValidateIdentityClaim(reply, a.cfg.Principal)
		}},
		{replyCheckResponseStyle, func() error {
			return ValidateResponseStyle(a.cfg.Definition.ResponseStyle, reply)
		}},
	}
	for _, check := range checks {
		if err := check.run(); err != nil {
			return reply, refusedBy(check.name, err), err
		}
	}
	return reply, replyCheckNone, nil
}
