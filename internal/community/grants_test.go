package community

import (
	"context"
	"strings"
	"testing"
	"time"
)

func grantTable() GrantTable {
	return GrantTable{Principals: []PrincipalGrant{
		{ID: "318190481467244544", Note: "the trusted principal", Kinds: Allowlist{All: true}},
		{ID: "222190481467244544", Note: "a guild member", Kinds: Allowlist{IDs: []string{"echo"}}},
	}}
}

// Absence is denial, matching the admission gate's fail-closed shape. See #150.
func TestAPrincipalWithNoEntryIsDenied(t *testing.T) {
	t.Parallel()
	table := grantTable()
	err := table.Permits("999999999999999999", "echo")
	if err == nil {
		t.Fatal("a principal with no grant entry was permitted")
	}
	if !IsGrantDenial(err) {
		t.Errorf("error is not a grant denial: %v", err)
	}
	if !strings.Contains(err.Error(), "no grant entry") {
		t.Errorf("denial does not say why: %v", err)
	}
	if err := table.Permits("", "echo"); err == nil {
		t.Error("an empty principal was permitted")
	}
}

// Two principals with different grants get different outcomes for one request.
func TestGrantsDifferByPrincipal(t *testing.T) {
	t.Parallel()
	table := grantTable()
	if err := table.Permits("318190481467244544", "ward-exec"); err != nil {
		t.Errorf("the all-grant principal was denied: %v", err)
	}
	err := table.Permits("222190481467244544", "ward-exec")
	if err == nil {
		t.Fatal("a narrowly granted principal reached ward-exec")
	}
	if !strings.Contains(err.Error(), "ward-exec") {
		t.Errorf("denial does not name the kind: %v", err)
	}
	if err := table.Permits("222190481467244544", "echo"); err != nil {
		t.Errorf("a granted kind was denied: %v", err)
	}
}

// A malformed table stops startup rather than silently denying everyone.
func TestAMalformedGrantTableIsRefused(t *testing.T) {
	t.Parallel()
	cases := []GrantTable{
		{Principals: []PrincipalGrant{{ID: "not-a-snowflake", Kinds: Allowlist{All: true}}}},
		{Principals: []PrincipalGrant{{ID: "318190481467244544"}}},
		{Principals: []PrincipalGrant{
			{ID: "318190481467244544", Kinds: Allowlist{IDs: []string{"deploy-everything"}}},
		}},
		{Principals: []PrincipalGrant{
			{ID: "318190481467244544", Kinds: Allowlist{All: true}},
			{ID: "318190481467244544", Kinds: Allowlist{All: true}},
		}},
	}
	for index, table := range cases {
		if err := table.validate(); err == nil {
			t.Errorf("case %d validated a malformed table", index)
		}
	}
	if err := grantTable().validate(); err != nil {
		t.Errorf("a well-formed table was refused: %v", err)
	}
}

// A denial is a recorded outcome with a reason, not a silent no-op.
func TestADeniedSubmissionIsRecordedAndExplained(t *testing.T) {
	t.Parallel()
	table := grantTable()
	runner := &JobRunner{
		Store:     NewMemoryJobStore(fixedClock(time.Unix(1700000000, 0).UTC())),
		Executors: map[string]JobExecutor{"echo": EchoJobExecutor{}},
		Grants:    &table,
	}
	if err := runner.Start(context.Background()); err != nil {
		t.Fatalf("Start: %v", err)
	}
	t.Cleanup(runner.Stop)

	submission := discordSubmission()
	submission.Principal = "999999999999999999"
	_, err := runner.Submit(context.Background(), submission)
	if err == nil {
		t.Fatal("an ungranted principal submitted a job")
	}
	if !IsGrantDenial(err) {
		t.Fatalf("error is not a grant denial: %v", err)
	}
	// The refusal is attributable rather than invisible.
	listed, listErr := runner.Store.ListByPrincipal(submission.Principal)
	if listErr != nil {
		t.Fatalf("ListByPrincipal: %v", listErr)
	}
	if len(listed) != 1 {
		t.Fatalf("a denial left %d records", len(listed))
	}
	if listed[0].State != JobFailed || listed[0].Outcome != "not permitted" {
		t.Errorf("denied job = %#v", listed[0])
	}
}

// A granted principal is unaffected, so the gate denies rather than breaks.
func TestAGrantedSubmissionStillRuns(t *testing.T) {
	t.Parallel()
	table := grantTable()
	runner := &JobRunner{
		Store:     NewMemoryJobStore(nil),
		Executors: map[string]JobExecutor{"echo": EchoJobExecutor{}},
		Grants:    &table,
		Timeout:   5 * time.Second,
	}
	if err := runner.Start(context.Background()); err != nil {
		t.Fatalf("Start: %v", err)
	}
	t.Cleanup(runner.Stop)

	job, err := runner.Submit(context.Background(), discordSubmission())
	if err != nil {
		t.Fatalf("a granted submission was refused: %v", err)
	}
	waitForState(t, runner, job.ID, job.Principal, JobSucceeded)
}

// No table grants everything, which is the posture before one is adopted.
func TestNoGrantTableGrantsEverything(t *testing.T) {
	t.Parallel()
	runner := &JobRunner{Store: NewMemoryJobStore(nil)}
	if err := runner.permits(Job{Principal: "anyone", Kind: "ward-exec"}); err != nil {
		t.Errorf("a nil table denied: %v", err)
	}
	if table := jobGrants(nil); table != nil {
		t.Error("a nil policy produced a table")
	}
	if table := jobGrants(&AccessPolicy{}); table != nil {
		t.Error("a policy declaring no principal produced a table")
	}
	policy := &AccessPolicy{Grants: grantTable()}
	if table := jobGrants(policy); table == nil {
		t.Error("a declared table was dropped")
	}
}

// A principal can be told what they hold rather than discovering it by refusal.
func TestGrantedKindsAreListable(t *testing.T) {
	t.Parallel()
	table := grantTable()
	narrow := table.GrantedKinds("222190481467244544")
	if len(narrow) != 1 || narrow[0] != "echo" {
		t.Errorf("narrow grants = %v", narrow)
	}
	broad := table.GrantedKinds("318190481467244544")
	if len(broad) != len(JobKinds) {
		t.Errorf("all-grant lists %v, want every kind", broad)
	}
	if got := table.GrantedKinds("999999999999999999"); got != nil {
		t.Errorf("an ungranted principal lists %v", got)
	}
}
