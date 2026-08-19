package community

import (
	"strings"
	"testing"
)

// Deep could not tell from a turn who it may answer, and both obvious readings
// were wrong. Rendered from the mounted policy. See sirens-echo#909.

// deepPolicy is the deployed sirens-deep shape: one guild, one channel, any
// member inside it, direct messages limited to one account.
func deepPolicy() *AccessPolicy {
	return &AccessPolicy{
		DirectMessages: DirectMessageAccess{Allow: []string{"318190481467244544"}},
		Guilds: []GuildAccess{{
			ID:       "1300204416229441587",
			Channels: Allowlist{IDs: []string{"1537024102886277210"}},
			Users:    Allowlist{All: true},
		}},
	}
}

func TestTheBoundSaysAnyMemberInOneChannelRatherThanEveryone(t *testing.T) {
	t.Parallel()
	bound := AdmissionBound(deepPolicy())
	for _, want := range []string{"Any member", "1 channel(s)", "1 guild(s)"} {
		if !strings.Contains(bound, want) {
			t.Fatalf("the bound %q is missing %q", bound, want)
		}
	}
	// The reading the issue was written under, which is the one to refuse.
	if strings.Contains(strings.ToLower(bound), "every channel") {
		t.Errorf("the bound claims wildcard reach: %q", bound)
	}
}

// A count of one account and the id of that account are different things to
// put in a prompt, and the identifier guard refuses a reply carrying the id.
func TestTheBoundCarriesNoIdentifiers(t *testing.T) {
	t.Parallel()
	policy := deepPolicy()
	policy.Agents = AgentAccess{Allow: []string{"wuf", "snow", "alpha"}}
	bound := AdmissionBound(policy)
	for _, secret := range []string{
		"318190481467244544", "1300204416229441587", "1537024102886277210",
		"wuf", "snow", "alpha",
	} {
		if strings.Contains(bound, secret) {
			t.Fatalf("the bound leaked the identifier %q: %q", secret, bound)
		}
	}
	if !strings.Contains(bound, "3 agent account(s)") {
		t.Errorf("the bound does not count the admitted agents: %q", bound)
	}
}

// The owl.glass shape: no direct messages at all, which must not read the same
// as one admitted account.
func TestRefusedDirectMessagesReadDifferentlyFromAdmittedOnes(t *testing.T) {
	t.Parallel()
	closed := AdmissionBound(&AccessPolicy{Guilds: []GuildAccess{{
		Channels: Allowlist{IDs: []string{"c"}}, Users: Allowlist{All: true},
	}}})
	if !strings.Contains(closed, "Direct messages are refused") {
		t.Errorf("an empty DM allowlist rendered %q", closed)
	}
	// The whole-string phrase, since the closing sentence also says "refused"
	// about messages outside the bound.
	if strings.Contains(AdmissionBound(deepPolicy()), "Direct messages are refused") {
		t.Errorf("an admitted DM account rendered as refused")
	}
	if !strings.Contains(AdmissionBound(deepPolicy()), "limited to one account") {
		t.Errorf("a single admitted DM account did not render as one")
	}
}

// A listed member set is a different bound from an open one, and collapsing
// them is the guess this issue exists to stop.
func TestAListedMemberSetDoesNotReadAsOpen(t *testing.T) {
	t.Parallel()
	listed := AdmissionBound(&AccessPolicy{Guilds: []GuildAccess{{
		Channels: Allowlist{IDs: []string{"c"}},
		Users:    Allowlist{IDs: []string{"u1", "u2"}},
	}}})
	if !strings.Contains(listed, "Only listed members") {
		t.Fatalf("a listed member set rendered as %q", listed)
	}
	if strings.Contains(listed, "Any member") {
		t.Errorf("a listed member set claims any member: %q", listed)
	}
}

// A deployment with no policy says nothing rather than guessing, and a caller
// with no policy must not crash the turn.
func TestNoPolicyRendersNothing(t *testing.T) {
	t.Parallel()
	if got := AdmissionBound(nil); got != "" {
		t.Errorf("a nil policy rendered %q", got)
	}
	if got := AdmissionBound(&AccessPolicy{}); !strings.Contains(got, "No guild") {
		t.Errorf("an empty policy rendered %q, which does not say it admits nothing", got)
	}
}
