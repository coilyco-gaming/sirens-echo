package community

import (
	"fmt"
	"strings"
)

// Execution runs under pod authority with no per-requester attribution.
// See docs/sirens-echo-execution.md.

// ExecutionAdmissionError explains why execution cannot be enabled against the
// current admission surface.
type ExecutionAdmissionError struct {
	Reason string
}

func (e ExecutionAdmissionError) Error() string {
	return "execution refused: " + e.Reason
}

// CheckExecutionAdmission refuses execution whenever the admission surface is
// wider than a single-account direct-message allowlist.
func CheckExecutionAdmission(policy *AccessPolicy) error {
	if policy == nil {
		return ExecutionAdmissionError{Reason: "no access policy is loaded"}
	}
	// A grant table is what the single-account rule stood in for.
	// See docs/sirens-echo-execution.md.
	if len(policy.Grants.Principals) > 0 {
		return checkExecutionGrants(policy)
	}
	if policy.legacyOpenDMs {
		return ExecutionAdmissionError{
			Reason: "direct messages are open to every account through the environment path",
		}
	}
	if len(policy.Agents.Allow) > 0 {
		return ExecutionAdmissionError{
			Reason: "a counterpart agent is admitted, so requesters are no longer one account",
		}
	}
	if len(policy.Guilds) > 0 || policy.catchAll != nil {
		return ExecutionAdmissionError{
			Reason: "a guild is admitted, so requesters are no longer one account",
		}
	}
	switch len(policy.DirectMessages.Allow) {
	case 1:
		return nil
	case 0:
		return ExecutionAdmissionError{Reason: "no account is admitted at all"}
	default:
		return ExecutionAdmissionError{
			Reason: fmt.Sprintf(
				"%d accounts are admitted, and execution carries no per-requester attribution",
				len(policy.DirectMessages.Allow),
			),
		}
	}
}

// checkExecutionGrants requires that execution be granted deliberately rather
// than inherited by a principal holding everything.
func checkExecutionGrants(policy *AccessPolicy) error {
	for _, grant := range policy.Grants.Principals {
		if grant.Kinds.Permits("ward-exec") {
			return nil
		}
	}
	return ExecutionAdmissionError{
		Reason: "no principal is granted ward-exec, so enabling execution would grant nobody anything",
	}
}

// ExecutionAdmissionSummary describes the current surface for an operator
// reading a startup log, without naming an account.
func ExecutionAdmissionSummary(policy *AccessPolicy) string {
	if policy == nil {
		return "no policy"
	}
	parts := []string{
		fmt.Sprintf("guilds=%d", len(policy.Guilds)),
		fmt.Sprintf("dm_accounts=%d", len(policy.DirectMessages.Allow)),
		fmt.Sprintf("granted_principals=%d", len(policy.Grants.Principals)),
	}
	if policy.legacyOpenDMs {
		parts = append(parts, "open_dms=true")
	}
	if policy.catchAll != nil {
		parts = append(parts, "catch_all=true")
	}
	return strings.Join(parts, " ")
}
