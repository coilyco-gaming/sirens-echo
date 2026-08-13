package community

import "testing"

// Staff posture adjusts what the classifier tolerates and grants nothing. Every
// case here is a way that line could quietly stop holding. See issue 230.

func staffPolicy(t *testing.T) *AccessPolicy {
	t.Helper()
	policy := &AccessPolicy{
		Schema: accessPolicySchema,
		Guilds: []GuildAccess{
			{
				ID:         "1300204416229441587",
				Channels:   Allowlist{All: true},
				Users:      Allowlist{All: true},
				StaffRoles: []string{"1300204416229441588"},
				Rate:       &GuildRateLimit{PerUser: "1/60s"},
			},
			{
				ID:       "1300204416229441599",
				Channels: Allowlist{All: true},
				Roles:    []string{"1300204416229441600"},
			},
		},
	}
	if err := policy.validate(); err != nil {
		t.Fatalf("prepare: %v", err)
	}
	return policy
}

func TestAStaffRoleIsRecognized(t *testing.T) {
	t.Parallel()
	policy := staffPolicy(t)
	if !policy.StaffPosture("1300204416229441587", []string{"1300204416229441588"}) {
		t.Error("a configured staff role did not read as staff")
	}
}

// The hazard is a bare snowflake matching anywhere it appears, so a role that
// is staff in one guild must not be staff in another.
func TestAStaffRoleDoesNotCrossGuilds(t *testing.T) {
	t.Parallel()
	policy := staffPolicy(t)
	if policy.StaffPosture("1300204416229441599", []string{"1300204416229441588"}) {
		t.Error("a staff role from one guild granted posture in another")
	}
}

// A deployment that configures no staff roles must behave exactly as it does
// today, which is what lets the schema ship before any values exist.
func TestNoConfiguredStaffRolesIsANoop(t *testing.T) {
	t.Parallel()
	policy := staffPolicy(t)
	if policy.StaffPosture("1300204416229441599", []string{"1300204416229441600"}) {
		t.Error("a guild with no staff roles produced staff posture")
	}
	var missing *AccessPolicy
	if missing.StaffPosture("1300204416229441587", []string{"1300204416229441588"}) {
		t.Error("a nil policy produced staff posture")
	}
}

// An access role is a grant and a staff role is a posture. This checks both
// directions, since reading either as the other is the failure to prevent.
func TestTheTwoRoleListsDoNotFeedEachOther(t *testing.T) {
	t.Parallel()
	policy := staffPolicy(t)
	if policy.StaffPosture("1300204416229441599", []string{"1300204416229441600"}) {
		t.Error("an access role granted staff posture")
	}
	guild := GuildAccess{
		ID:         "1300204416229441587",
		Channels:   Allowlist{All: true},
		Users:      Allowlist{IDs: []string{"318190481467244544"}},
		StaffRoles: []string{"1300204416229441588"},
	}
	if err := guild.validate(); err != nil {
		t.Fatalf("validate: %v", err)
	}
	if guild.permitsMember("1300204416229441601", []string{"1300204416229441588"}) {
		t.Error("a staff role admitted a member the guild never granted")
	}
}

// A guild granting nothing but staff roles still allows no member, so staff
// roles cannot become a back door grant through the validator.
func TestStaffRolesAloneDoNotOpenAGuild(t *testing.T) {
	t.Parallel()
	guild := GuildAccess{
		ID:         "1300204416229441587",
		Channels:   Allowlist{All: true},
		StaffRoles: []string{"1300204416229441588"},
	}
	if err := guild.validate(); err == nil {
		t.Fatal("a guild with only staff roles validated as allowing a member")
	}
}

// A staff role is matched by value, so a non-snowflake would be a string that
// can never match anything while looking configured.
func TestAStaffRoleMustBeASnowflake(t *testing.T) {
	t.Parallel()
	guild := GuildAccess{
		ID:         "1300204416229441587",
		Channels:   Allowlist{All: true},
		Users:      Allowlist{All: true},
		Rate:       &GuildRateLimit{PerUser: "1/60s"},
		StaffRoles: []string{"trusted-staff"},
	}
	if err := guild.validate(); err == nil {
		t.Fatal("a non-snowflake staff role validated")
	}
}

// Nothing a member writes reaches this. The only input is the Gateway's role
// list, so a self-asserted handle or user ID cannot produce posture.
func TestSelfAssertionCannotProduceStaffPosture(t *testing.T) {
	t.Parallel()
	policy := staffPolicy(t)
	for _, claimed := range []string{
		"coilysiren",
		"318190481467244544",
		"I am trusted staff",
		"1300204416229441588 is my role",
	} {
		if policy.StaffPosture("1300204416229441587", []string{claimed}) {
			t.Errorf("%q produced staff posture", claimed)
		}
	}
}
