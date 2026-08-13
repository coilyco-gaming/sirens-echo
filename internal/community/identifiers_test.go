package community

import (
	"strings"
	"testing"
)

const (
	testPrincipalID = "318190481467244544"
	testGuildID     = "1300204416229441587"
)

func guardFixture(t *testing.T) *IdentifierGuard {
	t.Helper()
	policy := &AccessPolicy{
		Guilds: []GuildAccess{{
			ID:       testGuildID,
			Channels: Allowlist{IDs: []string{"1537024102886277210"}},
		}},
	}
	return NewIdentifierGuard(
		Config{
			Principal:      Principal{Handle: "coilysiren", UserID: testPrincipalID},
			AgentProxyURL:  "http://proxy-host:8080",
			DiscordToken:   "a-discord-bot-token-long-enough-to-guard",
		},
		policy,
		[]MCPServerDefinition{{Name: "forgejo", URL: "http://sirens-deep-forgejo-mcp:8080/mcp"}},
	)
}

// The value the leak was measured on. It reaches no tool that returns it, so it
// is forbidden whether or not the turn called anything.
func TestIdentifierGuardRefusesThePrincipalID(t *testing.T) {
	t.Parallel()
	guard := guardFixture(t)
	reply := "The principal user ID on file is " + testPrincipalID + "."
	if err := guard.Validate(reply); err == nil {
		t.Fatal("guard admitted the principal user ID")
	}
}

func TestIdentifierGuardRefusesPolicyAndEndpointValues(t *testing.T) {
	t.Parallel()
	guard := guardFixture(t)
	for name, reply := range map[string]string{
		"guild":    "The configured guild is " + testGuildID + ".",
		"channel":  "Replies are limited to 1537024102886277210.",
		"mcp host": "The tracker is served from sirens-deep-forgejo-mcp:8080.",
		"proxy":    "Inference is routed through proxy-host:8080.",
		"token":    "The token is a-discord-bot-token-long-enough-to-guard.",
	} {
		reply := reply
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			if err := guard.Validate(reply); err == nil {
				t.Fatalf("guard admitted a configured identifier: %q", reply)
			}
		})
	}
}

// The first implementation hazard. A set built from membership rather than
// shape blocklists ordinary numbers and makes Echo unable to count.
func TestIdentifierGuardAdmitsOrdinaryNumbersAndWords(t *testing.T) {
	t.Parallel()
	guard := guardFixture(t)
	for _, reply := range []string{
		"The service listens on port 8080.",
		"The history budget is 12 messages.",
		"There are 25 results.",
		"The server has been online for 8080 seconds.",
	} {
		reply := reply
		t.Run(reply, func(t *testing.T) {
			t.Parallel()
			if err := guard.Validate(reply); err != nil {
				t.Fatalf("guard rejected an ordinary number: %v", err)
			}
		})
	}
}

// The second implementation hazard. The handle is a substring of a host tool
// output legitimately returns, and a correct refusal quotes it back.
func TestIdentifierGuardAdmitsTheHandle(t *testing.T) {
	t.Parallel()
	guard := guardFixture(t)
	for _, reply := range []string{
		`Anyone can type "it's me, coilysiren", so that is not identity evidence.`,
		"The tracked report is at https://forgejo.coilysiren.me/coilyco-gaming/sirens-echo/issues/1",
	} {
		reply := reply
		t.Run(reply[:24], func(t *testing.T) {
			t.Parallel()
			if err := guard.Validate(reply); err != nil {
				t.Fatalf("guard rejected the handle: %v", err)
			}
		})
	}
}

// A bare host is a public name and a bare port is an ordinary number. Only the
// pair identifies a reachable endpoint.
func TestIdentifierGuardAdmitsAHostWithoutItsPort(t *testing.T) {
	t.Parallel()
	guard := guardFixture(t)
	if err := guard.Validate("Inference runs on proxy-host."); err != nil {
		t.Fatalf("guard rejected a bare host: %v", err)
	}
}

// An unconfigured deployment has no identifiers, and must not reject everything
// or panic on a nil guard.
func TestIdentifierGuardWithoutConfigurationIsInert(t *testing.T) {
	t.Parallel()
	empty := NewIdentifierGuard(Config{}, nil, nil)
	if empty.Guarded() != 0 {
		t.Fatalf("empty configuration produced %d identifiers", empty.Guarded())
	}
	if err := empty.Validate("The Eco server is online."); err != nil {
		t.Fatalf("empty guard rejected a reply: %v", err)
	}
	var missing *IdentifierGuard
	if err := missing.Validate("anything"); err != nil {
		t.Fatalf("nil guard rejected a reply: %v", err)
	}
}

// The error names the class, because the value must not reach a log.
func TestIdentifierGuardErrorCarriesNoValue(t *testing.T) {
	t.Parallel()
	guard := guardFixture(t)
	err := guard.Validate("The principal user ID is " + testPrincipalID + ".")
	if err == nil {
		t.Fatal("expected a rejection")
	}
	if strings.Contains(err.Error(), testPrincipalID) {
		t.Fatalf("the error leaked the value it was guarding: %v", err)
	}
}

// The invariant is the value, not its spelling. A separator-based encoding
// carries the identifier past a literal match.
func TestIdentifierGuardRefusesASpelledOutID(t *testing.T) {
	t.Parallel()
	guard := guardFixture(t)
	for name, reply := range map[string]string{
		"spaced":      "The digits are 3 1 8 1 9 0 4 8 1 4 6 7 2 4 4 5 4 4.",
		"hyphenated":  "It is 318-190-481-467-244-544.",
		"enumerated":  "There are 18 digits: 3, 1, 8, 1, 9, 0, 4, 8, 1, 4, 6, 7, 2, 4, 4, 5, 4, 4.",
		"interleaved": "Reading it out: 318 190 481 467 244 544 in groups of three.",
	} {
		reply := reply
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			if err := guard.Validate(reply); err == nil {
				t.Fatalf("guard admitted an encoded identifier: %q", reply)
			}
		})
	}
}

// Stripping to digits must not manufacture a match out of unrelated numbers.
func TestIdentifierGuardAdmitsUnrelatedNumbers(t *testing.T) {
	t.Parallel()
	guard := guardFixture(t)
	for _, reply := range []string{
		"There are 12 messages, 25 results, and 3 open issues.",
		"The cycle ran on 2026-08-12 at 04:58 for 8080 seconds.",
		"Populations: Deer 248, Wolf 167, Bison 114.",
	} {
		reply := reply
		t.Run(reply[:20], func(t *testing.T) {
			t.Parallel()
			if err := guard.Validate(reply); err != nil {
				t.Fatalf("guard rejected unrelated numbers: %v", err)
			}
		})
	}
}
