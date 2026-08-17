package community

import (
	"strings"
)

// A block is model text on a path the reply checks never run.
// See docs/sirens-echo-content-gate.md.

const (
	// blockRedirect is the whole response for a sensitive class, and the
	// fallback whenever a reason cannot be trusted.
	blockRedirect = "That is outside what this service answers."
)

// BlockResponse renders a refusal a member can act on without learning what to
// avoid saying next time.
func BlockResponse(class ContentClass, reason string, principal Principal) string {
	// A sensitive class names nothing and carries no model text at all. A
	// reason here is redundant at best and leaks the hidden signal at worst.
	if class.Sensitive {
		return blockRedirect
	}
	trimmed := boundedBlockReason(reason, principal)
	if trimmed == "" {
		return blockRedirect
	}
	return trimmed
}

// boundedBlockReason returns the reason only when every reply check the normal
// path applies also passes here. Anything else falls back rather than ships.
func boundedBlockReason(reason string, principal Principal) string {
	reason = strings.TrimSpace(collapseBlockLines(reason))
	if reason == "" {
		return ""
	}
	if len(strings.Fields(reason)) > maxBlockReasonWords {
		return ""
	}
	// The same guards the reply path runs, on text that would otherwise reach a
	// member having passed none of them.
	if PrincipalEchoed(reason, principal) {
		return ""
	}
	if ValidateIdentityClaim(reason, principal) != nil {
		return ""
	}
	if ValidateNeutralStyle(reason) != nil {
		return ""
	}
	return reason
}

// collapseBlockLines keeps a refusal to one line. A multi-line reason reads as
// an argument, and an argument invites the next message.
func collapseBlockLines(reason string) string {
	return strings.Join(strings.Fields(reason), " ")
}
