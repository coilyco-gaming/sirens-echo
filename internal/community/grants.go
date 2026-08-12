package community

import (
	"errors"
	"fmt"
	"sort"
	"strings"
)

// Filtered grants over one credential: the harness narrows what a principal may
// cause, and the pod keeps its own identity. See docs/sirens-echo-grants.md.

// PrincipalGrant is one requester's permitted job kinds. It is a document
// entry, so the grant model is reviewable rather than inferred from code.
type PrincipalGrant struct {
	// ID is the Discord user ID, the stable opaque identifier attribution
	// already records.
	ID string `yaml:"id"`
	// Note is a review aid and is never matched on.
	Note string `yaml:"note"`
	// Kinds are the job kinds this principal may cause. `all` is permitted and
	// deliberately conspicuous in a diff.
	Kinds Allowlist `yaml:"kinds"`
}

// GrantTable is the deployment-owned per-principal grant model.
type GrantTable struct {
	Principals []PrincipalGrant `yaml:"principals"`
}

// GrantDenial explains a refusal. It is a defined outcome with a stated reason,
// never a crash and never a silent no-op.
type GrantDenial struct {
	Principal string
	Kind      string
	Reason    string
}

func (d GrantDenial) Error() string {
	return "not permitted: " + d.Reason
}

// IsGrantDenial reports a refusal a caller should record rather than retry.
func IsGrantDenial(err error) bool {
	var denial GrantDenial
	return asGrantDenial(err, &denial)
}

// validate rejects a table the runtime must never enforce, so a malformed grant
// stops startup instead of silently denying everyone.
func (g GrantTable) validate() error {
	seen := make(map[string]struct{}, len(g.Principals))
	for _, grant := range g.Principals {
		if !discordSnowflake.MatchString(grant.ID) {
			return fmt.Errorf("grant IDs must be numeric snowflakes, got %q", grant.ID)
		}
		if _, duplicate := seen[grant.ID]; duplicate {
			return fmt.Errorf("grant table lists principal %s twice", grant.ID)
		}
		seen[grant.ID] = struct{}{}
		if grant.Kinds.empty() {
			return fmt.Errorf("principal %s grants no kind: use `all` or list kinds", grant.ID)
		}
		for _, kind := range grant.Kinds.IDs {
			if _, known := JobKinds[kind]; !known {
				return fmt.Errorf("principal %s is granted unknown job kind %q", grant.ID, kind)
			}
		}
	}
	return nil
}

// Permits decides what a principal may cause. Absence is denial, matching the
// admission gate's fail-closed shape.
func (g GrantTable) Permits(principal, kind string) error {
	if strings.TrimSpace(principal) == "" {
		return GrantDenial{Kind: kind, Reason: "no requesting principal"}
	}
	for _, grant := range g.Principals {
		if grant.ID != principal {
			continue
		}
		if grant.Kinds.Permits(kind) {
			return nil
		}
		return GrantDenial{
			Principal: principal,
			Kind:      kind,
			Reason:    fmt.Sprintf("principal is not granted %s", kind),
		}
	}
	return GrantDenial{
		Principal: principal,
		Kind:      kind,
		Reason:    "principal has no grant entry",
	}
}

// GrantedKinds lists what a principal may cause, for answering the question
// without making them discover it by being refused.
func (g GrantTable) GrantedKinds(principal string) []string {
	for _, grant := range g.Principals {
		if grant.ID != principal {
			continue
		}
		if grant.Kinds.All {
			kinds := make([]string, 0, len(JobKinds))
			for kind := range JobKinds {
				kinds = append(kinds, kind)
			}
			sort.Strings(kinds)
			return kinds
		}
		granted := append([]string(nil), grant.Kinds.IDs...)
		sort.Strings(granted)
		return granted
	}
	return nil
}

func asGrantDenial(err error, target *GrantDenial) bool {
	return errors.As(err, target)
}
