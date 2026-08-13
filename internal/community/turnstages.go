package community

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
		// reads it verbatim. See docs/sirens-echo-capability-limits.md.
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
			return reply, check.name, err
		}
	}
	return reply, replyCheckNone, nil
}
