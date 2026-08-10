package community

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

const (
	testHomeGuild    = "2048000000000000001"
	testForeignGuild = "3072000000000000001"
	testOpenChannel  = "4000000000000000001"
	testNamedChannel = "4096000000000000001"
	testMember       = "1024000000000000001"
	testStranger     = "1024000000000000009"
	testModRole      = "5120000000000000001"
	testBlocked      = "9990000000000000001"
)

func writePolicy(t *testing.T, body string) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "access.yaml")
	if err := os.WriteFile(path, []byte(body), 0o600); err != nil {
		t.Fatalf("write policy: %v", err)
	}
	return path
}

func guildOrigin(guild, channel string) summonContext {
	return summonContext{Kind: contextKindGuild, GuildID: guild, ChannelID: channel}
}

// The reference copy is tracked documentation, so a schema change that breaks
// it must break the suite rather than reach a deployment.
func TestReferencePolicyParses(t *testing.T) {
	t.Parallel()
	policy, err := LoadAccessPolicy(
		filepath.Join("..", "..", "docs", "access-policy.reference.yaml"),
	)
	if err != nil {
		t.Fatalf("LoadAccessPolicy: %v", err)
	}
	if len(policy.Guilds) != 2 {
		t.Fatalf("guilds = %d, want 2", len(policy.Guilds))
	}
}

func TestAccessPolicyStacksGuildChannelAndMember(t *testing.T) {
	t.Parallel()
	path := writePolicy(t, `
schema: coilyco-harness.access.v1
deny:
  users: ["`+testBlocked+`"]
direct_messages:
  allow: ["`+testMember+`"]
guilds:
  - id: "`+testHomeGuild+`"
    note: "open"
    channels: all
    users: all
  - id: "`+testForeignGuild+`"
    note: "restricted"
    channels: ["`+testNamedChannel+`"]
    users: ["`+testMember+`"]
    roles: ["`+testModRole+`"]
`)
	policy, err := LoadAccessPolicy(path)
	if err != nil {
		t.Fatalf("LoadAccessPolicy: %v", err)
	}

	for _, testCase := range []struct {
		name   string
		origin summonContext
		user   string
		roles  []string
		want   accessReason
	}{
		{
			name:   "open guild admits any member and channel",
			origin: guildOrigin(testHomeGuild, testOpenChannel),
			user:   testStranger,
			want:   accessAllowed,
		},
		{
			name:   "restricted guild admits the listed member in the listed channel",
			origin: guildOrigin(testForeignGuild, testNamedChannel),
			user:   testMember,
			want:   accessAllowed,
		},
		{
			name:   "a role grant covers a member who is not listed",
			origin: guildOrigin(testForeignGuild, testNamedChannel),
			user:   testStranger,
			roles:  []string{testModRole},
			want:   accessAllowed,
		},
		{
			name:   "restricted guild refuses an unlisted member",
			origin: guildOrigin(testForeignGuild, testNamedChannel),
			user:   testStranger,
			want:   accessDeniedMember,
		},
		{
			name:   "restricted guild refuses an unlisted channel",
			origin: guildOrigin(testForeignGuild, testOpenChannel),
			user:   testMember,
			want:   accessNeedsThreadRef,
		},
		{
			name:   "an unlisted guild is refused outright",
			origin: guildOrigin("7000000000000000001", testOpenChannel),
			user:   testMember,
			want:   accessDeniedGuild,
		},
		{
			name:   "the deny list beats every allow rule",
			origin: guildOrigin(testHomeGuild, testOpenChannel),
			user:   testBlocked,
			want:   accessDeniedBlocked,
		},
		{
			name:   "a listed account may send a direct message",
			origin: summonContext{Kind: contextKindDM, ChannelID: "dm-1"},
			user:   testMember,
			want:   accessAllowed,
		},
		{
			name:   "an unlisted account may not",
			origin: summonContext{Kind: contextKindDM, ChannelID: "dm-1"},
			user:   testStranger,
			want:   accessDeniedDM,
		},
	} {
		got := policy.Evaluate(testCase.origin, testCase.user, testCase.roles, nil)
		if got.Reason != testCase.want {
			t.Errorf("%s: reason = %q, want %q", testCase.name, got.Reason, testCase.want)
		}
	}
}

// A blocked member must be refused before the guild is even consulted, so the
// deny list works in a guild the operator cannot moderate.
func TestAccessPolicyDenyBeatsRoleGrant(t *testing.T) {
	t.Parallel()
	path := writePolicy(t, `
schema: coilyco-harness.access.v1
deny:
  users: ["`+testBlocked+`"]
guilds:
  - id: "`+testHomeGuild+`"
    channels: all
    roles: ["`+testModRole+`"]
`)
	policy, err := LoadAccessPolicy(path)
	if err != nil {
		t.Fatalf("LoadAccessPolicy: %v", err)
	}
	got := policy.Evaluate(
		guildOrigin(testHomeGuild, testOpenChannel),
		testBlocked,
		[]string{testModRole},
		nil,
	)
	if got.Reason != accessDeniedBlocked {
		t.Fatalf("reason = %q, want %q", got.Reason, accessDeniedBlocked)
	}
}

func TestAccessPolicyCarriesPerGuildRateOverrides(t *testing.T) {
	t.Parallel()
	path := writePolicy(t, `
schema: coilyco-harness.access.v1
guilds:
  - id: "`+testHomeGuild+`"
    channels: all
    users: all
  - id: "`+testForeignGuild+`"
    channels: all
    users: all
    rate_limit:
      per_user: "2/60s"
`)
	policy, err := LoadAccessPolicy(path)
	if err != nil {
		t.Fatalf("LoadAccessPolicy: %v", err)
	}
	home := policy.Evaluate(guildOrigin(testHomeGuild, testOpenChannel), testMember, nil, nil)
	if home.Guild.Overrides() != nil {
		t.Fatal("a guild without rate_limit must keep the deployment default")
	}
	foreign := policy.Evaluate(guildOrigin(testForeignGuild, testOpenChannel), testMember, nil, nil)
	override := foreign.Guild.Overrides()
	if override == nil || override.PerUser == nil {
		t.Fatalf("override = %#v, want a per-user limit", override)
	}
	if override.PerUser.Burst != 2 || override.PerUser.Every != time.Minute {
		t.Fatalf("per-user override = %#v", *override.PerUser)
	}
	if override.PerContext != nil {
		t.Fatal("an unset tier must not be overridden")
	}
}

// Every rejection here is a case where accepting the file would widen the
// surface it was written to narrow.
func TestAccessPolicyFailsClosed(t *testing.T) {
	t.Parallel()
	for _, testCase := range []struct {
		name string
		body string
		want string
	}{
		{
			name: "wrong schema",
			body: "schema: something.else\nguilds: [{id: \"" + testHomeGuild + "\", channels: all, users: all}]",
			want: "unsupported access policy schema",
		},
		{
			name: "grants nothing",
			body: "schema: coilyco-harness.access.v1\nguilds: []",
			want: "grants nothing",
		},
		{
			name: "channel name instead of an ID",
			body: "schema: coilyco-harness.access.v1\nguilds: [{id: \"" + testHomeGuild + "\", channels: [\"#bots\"], users: all}]",
			want: "numeric snowflakes",
		},
		{
			name: "guild allows no member",
			body: "schema: coilyco-harness.access.v1\nguilds: [{id: \"" + testHomeGuild + "\", channels: all}]",
			want: "allows no member",
		},
		{
			name: "guild allows no channel",
			body: "schema: coilyco-harness.access.v1\nguilds: [{id: \"" + testHomeGuild + "\", users: all}]",
			want: "allows no channel",
		},
		{
			name: "duplicate guild",
			body: "schema: coilyco-harness.access.v1\nguilds:\n  - {id: \"" + testHomeGuild + "\", channels: all, users: all}\n  - {id: \"" + testHomeGuild + "\", channels: all, users: all}",
			want: "twice",
		},
		{
			name: "misspelled widening token",
			body: "schema: coilyco-harness.access.v1\nguilds: [{id: \"" + testHomeGuild + "\", channels: any, users: all}]",
			want: "expected `all`",
		},
		{
			name: "unknown field",
			body: "schema: coilyco-harness.access.v1\nguilds: [{id: \"" + testHomeGuild + "\", channel: all, users: all}]",
			want: "field channel not found",
		},
		{
			name: "malformed rate override",
			body: "schema: coilyco-harness.access.v1\nguilds: [{id: \"" + testHomeGuild + "\", channels: all, users: all, rate_limit: {per_user: \"soon\"}}]",
			want: "rate_limit",
		},
	} {
		_, err := LoadAccessPolicy(writePolicy(t, testCase.body))
		if err == nil {
			t.Errorf("%s: policy was accepted", testCase.name)
			continue
		}
		if !strings.Contains(err.Error(), testCase.want) {
			t.Errorf("%s: error = %v, want it to mention %q", testCase.name, err, testCase.want)
		}
	}

	if _, err := LoadAccessPolicy(filepath.Join(t.TempDir(), "absent.yaml")); err == nil {
		t.Fatal("a missing policy file was accepted")
	}
}

// The synthesized policy keeps one runtime representation, so the gate has a
// single code path whether or not a deployment has adopted the file.
func TestSynthesizedPolicyMatchesLegacyEnvironment(t *testing.T) {
	t.Parallel()
	anyGuild := synthesizeAccessPolicy(Config{
		DiscordChannelIDs: []string{testNamedChannel},
	})
	if got := anyGuild.Evaluate(guildOrigin(testForeignGuild, testNamedChannel), testStranger, nil, nil); !got.allowed() {
		t.Fatalf("channel-only scope: reason = %q, want allowed in any guild", got.Reason)
	}
	if got := anyGuild.Evaluate(summonContext{Kind: contextKindDM, ChannelID: "dm-1"}, testMember, nil, nil); got.allowed() {
		t.Fatal("direct messages must stay off without the switch")
	}

	restricted := synthesizeAccessPolicy(Config{
		DiscordChannelIDs: []string{testNamedChannel},
		DiscordGuildIDs:   []string{testHomeGuild},
		DiscordDMEnabled:  true,
	})
	if got := restricted.Evaluate(guildOrigin(testForeignGuild, testNamedChannel), testMember, nil, nil); got.Reason != accessDeniedGuild {
		t.Fatalf("guild allowlist: reason = %q, want %q", got.Reason, accessDeniedGuild)
	}
	if got := restricted.Evaluate(guildOrigin(testHomeGuild, testNamedChannel), testMember, nil, nil); !got.allowed() {
		t.Fatalf("allowlisted guild: reason = %q, want allowed", got.Reason)
	}
	if got := restricted.Evaluate(summonContext{Kind: contextKindDM, ChannelID: "dm-1"}, testStranger, nil, nil); !got.allowed() {
		t.Fatal("the environment switch had no per-account list, so it admits any account")
	}
}
