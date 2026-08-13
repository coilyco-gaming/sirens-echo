package community

import (
	"strings"
	"testing"
)

const createdIssueResult = `{"result":{"body":"reported by a member","html_url":` +
	`"https://forgejo.coilysiren.me/coilyco-gaming/sirens-echo/issues/233#issue-8117",` +
	`"number":233,"url":"https://forgejo.coilysiren.me/api/v1/repos/coilyco-gaming/sirens-echo/issues/233"}}`

func createIssueCall() ExecutedTool {
	return ExecutedTool{
		Name:      "sirens-echo-forgejo__create_issue",
		Arguments: `{"title":"correction"}`,
		Result:    createdIssueResult,
	}
}

const wantIssue233 = "https://forgejo.coilysiren.me/coilyco-gaming/sirens-echo/issues/233"

// The reported defect: the model names the issue in short form only.
func TestAppendIssueReferencesResolvesShortForm(t *testing.T) {
	reply := "A correction issue has been filed for review (issue #233)."
	got := AppendIssueReferences(reply, createIssueCall())
	if !strings.HasPrefix(got, reply) {
		t.Fatalf("model reply was not preserved: %q", got)
	}
	if !strings.Contains(got, referenceHeading) {
		t.Fatalf("reference block missing: %q", got)
	}
	if !strings.Contains(got, wantIssue233) {
		t.Fatalf("issue url missing: %q", got)
	}
}

// The anchored form the API returns is not the durable link.
func TestAppendIssueReferencesDropsURLFragment(t *testing.T) {
	got := AppendIssueReferences("Filed for review.", createIssueCall())
	if strings.Contains(got, "#issue-8117") {
		t.Fatalf("fragment survived: %q", got)
	}
}

// An API URL is not a link a member can follow.
func TestAppendIssueReferencesSkipsAPIURL(t *testing.T) {
	got := AppendIssueReferences("Filed for review.", createIssueCall())
	if strings.Contains(got, "/api/v1/") {
		t.Fatalf("api url leaked: %q", got)
	}
}

// An issue filed without being mentioned still has to reach the member.
func TestAppendIssueReferencesLinksUnmentionedFiling(t *testing.T) {
	got := AppendIssueReferences("The plot detail is unverified.", createIssueCall())
	if !strings.Contains(got, wantIssue233) {
		t.Fatalf("silent filing was not linked: %q", got)
	}
}

// A reply that already carries the URL needs no repair.
func TestAppendIssueReferencesLeavesLinkedReplyAlone(t *testing.T) {
	reply := "A correction was filed: " + wantIssue233
	if got := AppendIssueReferences(reply, createIssueCall()); got != reply {
		t.Fatalf("linked reply was rewritten: %q", got)
	}
}

// No tool call means no observed URL, so there is nothing to ground a repair on.
func TestAppendIssueReferencesWithoutToolsIsInert(t *testing.T) {
	reply := "A correction has been filed for review (issue #233)."
	if got := AppendIssueReferences(reply); got != reply {
		t.Fatalf("ungrounded reference was invented: %q", got)
	}
}

// A number the turn never observed is not a reference this service can resolve.
func TestAppendIssueReferencesIgnoresUnobservedNumber(t *testing.T) {
	reply := "See issue #999 for the prior report."
	if got := AppendIssueReferences(reply, createIssueCall()); strings.Contains(got, "999") &&
		strings.Contains(got, referenceHeading+"\nhttps") &&
		strings.Contains(got, "/issues/999") {
		t.Fatalf("unobserved issue was linked: %q", got)
	}
}

// A read-only lookup grounds a short form just as well as a filing does.
func TestAppendIssueReferencesResolvesFromLookup(t *testing.T) {
	lookup := ExecutedTool{
		Name:   "sirens-echo-forgejo__get_issue",
		Result: `{"result":{"html_url":"https://forgejo.coilysiren.me/coilyco-gaming/sirens-echo/issues/57","number":57}}`,
	}
	got := AppendIssueReferences("The latest open issue is #57.", lookup)
	if !strings.Contains(got, "/issues/57") {
		t.Fatalf("lookup url missing: %q", got)
	}
}

// A channel mention is not an issue reference.
func TestAppendIssueReferencesIgnoresChannelMention(t *testing.T) {
	got := AppendIssueReferences("Replies stay in #bots.", createIssueCall())
	if strings.Count(got, wantIssue233) != 1 {
		t.Fatalf("channel mention disturbed the block: %q", got)
	}
}

// Two filings in one turn both belong in the block, once each.
func TestAppendIssueReferencesDeduplicates(t *testing.T) {
	got := AppendIssueReferences("Filed for review.", createIssueCall(), createIssueCall())
	if strings.Count(got, wantIssue233) != 1 {
		t.Fatalf("duplicate url: %q", got)
	}
}

// A block that cannot fit the send budget is dropped, not truncated into a
// broken URL by the transport.
func TestAppendIssueReferencesRespectsSendBudget(t *testing.T) {
	reply := strings.Repeat("a", discordReplyLimit)
	if got := AppendIssueReferences(reply, createIssueCall()); got != reply {
		t.Fatalf("oversized reply was extended: len=%d", len([]rune(got)))
	}
}

// The appended block is service-authored, so it must satisfy the same neutral
// restrictions the model reply was just checked against.
func TestAppendIssueReferencesKeepsNeutralStyle(t *testing.T) {
	got := AppendIssueReferences("The plot detail is unverified.", createIssueCall())
	if err := ValidateNeutralStyle(got); err != nil {
		t.Fatalf("appended block breaks neutral style: %v", err)
	}
}

// The append path resolves a short reference the model wrote, so the model has
// to know which references are safe to write. Nothing else states the rule.
func TestTheModelIsToldWhichIssueReferencesAreSafe(t *testing.T) {
	t.Parallel()
	prose := policyRootProse(t)
	for _, phrase := range []string{
		"A tracked issue is named by number, not by URL",
		"Name one only when a tool result this turn returned it",
	} {
		if !strings.Contains(prose, phrase) {
			t.Errorf("no policy root tells the model %q", phrase)
		}
	}
}

// A bare number is ambiguous across repositories, and a tool result can quote a
// sibling repository's issue. Linking either one is a guess.
func TestACollidingNumberIsSuppressedRatherThanGuessed(t *testing.T) {
	t.Parallel()
	quoted := ExecutedTool{Name: "forgejo__get_issue", Result: `{"result":{
	 "html_url":"https://forgejo.coilysiren.me/coilyco-bridge/deploy/issues/425",
	 "body":"see https://forgejo.coilysiren.me/coilyco-gaming/sirens-echo/issues/425"}}`}
	got := AppendIssueReferences("Tracked as #425.", quoted)
	if strings.Contains(got, "/issues/425") {
		t.Errorf("a colliding number was linked anyway:\n%s", got)
	}
}

// Suppression is for a genuine conflict only. The same issue quoted twice is
// one observation, and dropping it would lose the case this feature exists for.
func TestARepeatedObservationIsNotACollision(t *testing.T) {
	t.Parallel()
	repeated := ExecutedTool{Name: "forgejo__get_issue", Result: `{"result":{
	 "html_url":"https://forgejo.coilysiren.me/coilyco-gaming/sirens-echo/issues/233",
	 "body":"duplicate of https://forgejo.coilysiren.me/coilyco-gaming/sirens-echo/issues/233"}}`}
	got := AppendIssueReferences("Tracked as #233.", repeated)
	if !strings.Contains(got, "coilyco-gaming/sirens-echo/issues/233") {
		t.Errorf("a repeated observation was treated as a conflict:\n%s", got)
	}
}
