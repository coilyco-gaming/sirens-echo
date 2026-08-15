package community

import (
	"reflect"
	"testing"
)

// The model never supplies this label and cannot omit it. See sirens-echo#208.

func sandboxPolicy() issueLabelPolicy {
	return issueLabelPolicy{Tracker: "forgejo", SandboxID: 353}
}

// The default applies nothing, so landing this changes no deployment until one
// configures a label.
func TestNoConfiguredLabelAppliesNothing(t *testing.T) {
	t.Parallel()
	for name, policy := range map[string]issueLabelPolicy{
		"no label":   {Tracker: "forgejo"},
		"no tracker": {SandboxID: 353},
		"neither":    {},
		"negative":   {Tracker: "forgejo", SandboxID: -1},
	} {
		if policy.active() {
			t.Errorf("%s reported itself active", name)
		}
		if policy.applies(registeredMCPTool{serverName: "forgejo", toolName: issueLabelTool}) {
			t.Errorf("%s applied to a create-issue call", name)
		}
	}
}

func TestItAppliesOnlyToFilingOnTheTracker(t *testing.T) {
	t.Parallel()
	policy := sandboxPolicy()
	if !policy.applies(registeredMCPTool{serverName: "forgejo", toolName: issueLabelTool}) {
		t.Error("filing on the tracker was not labelled")
	}
	for name, tool := range map[string]registeredMCPTool{
		"another server": {serverName: "eco", toolName: issueLabelTool},
		"another verb":   {serverName: "forgejo", toolName: "comment_issue"},
		"a read":         {serverName: "forgejo", toolName: "get_issue"},
		"nothing named":  {},
	} {
		if policy.applies(tool) {
			t.Errorf("%s was labelled and should not have been", name)
		}
	}
}

func TestTheLabelIsSetRatherThanTrusted(t *testing.T) {
	t.Parallel()
	// A model-supplied value is replaced, not merged. It does not supply this
	// field, and one it invented is not a reason to keep it.
	got := sandboxPolicy().withHarnessLabels(map[string]any{
		"title":         "a report",
		"body":          "from a member",
		issueLabelField: []int{999},
	})
	if !reflect.DeepEqual(got[issueLabelField], []int{353}) {
		t.Errorf("labels = %v, want only the configured id", got[issueLabelField])
	}
	if got["title"] != "a report" || got["body"] != "from a member" {
		t.Errorf("the call's own arguments were altered: %v", got)
	}
}

// Copying rather than mutating matters because a retry must see what the model
// wrote, not what a previous attempt rewrote.
func TestItCopiesRatherThanMutates(t *testing.T) {
	t.Parallel()
	original := map[string]any{"title": "a report", "body": "from a member"}
	sandboxPolicy().withHarnessLabels(original)
	if _, present := original[issueLabelField]; present {
		t.Error("the caller's arguments were mutated")
	}
}

// Two labels at creation, which single-label was the working behaviour for.
// A second call afterwards would leave a window where the issue is unlabelled.
func TestCreationCarriesEveryConfiguredLabel(t *testing.T) {
	t.Parallel()
	policy := issueLabelPolicy{Tracker: "forgejo", SandboxID: 353, DestinationID: 358}
	got := policy.withHarnessLabels(map[string]any{"title": "a report"})
	if !reflect.DeepEqual(got[issueLabelField], []int{353, 358}) {
		t.Errorf("labels = %v, want both configured ids", got[issueLabelField])
	}
}

// The destination is a default rather than a replacement, so a deployment that
// knows the home sets it and the unknown label is never the one applied.
func TestTheDestinationIsWhicheverOneIsConfigured(t *testing.T) {
	t.Parallel()
	unknown := issueLabelPolicy{Tracker: "forgejo", SandboxID: 353, DestinationID: 358}
	known := issueLabelPolicy{Tracker: "forgejo", SandboxID: 353, DestinationID: 355}
	if reflect.DeepEqual(unknown.labels(), known.labels()) {
		t.Fatal("a known destination is indistinguishable from an unknown one")
	}
	// Exactly one move-to-repo label is sent, so the two never arrive together.
	if len(known.labels()) != 2 {
		t.Errorf("labels = %v, want the sandbox label and one destination", known.labels())
	}
}

// Each label is independently optional, so a deployment configuring one of the
// two is not silently given the other or refused both.
func TestEitherLabelAloneStillApplies(t *testing.T) {
	t.Parallel()
	for name, probe := range map[string]struct {
		policy issueLabelPolicy
		want   []int
	}{
		"sandbox only":     {issueLabelPolicy{Tracker: "forgejo", SandboxID: 353}, []int{353}},
		"destination only": {issueLabelPolicy{Tracker: "forgejo", DestinationID: 358}, []int{358}},
	} {
		if !probe.policy.active() {
			t.Errorf("%s reported itself inactive", name)
		}
		if got := probe.policy.labels(); !reflect.DeepEqual(got, probe.want) {
			t.Errorf("%s labels = %v, want %v", name, got, probe.want)
		}
	}
}

// The control the sandbox label exists for survives the widening. A model that
// names a destination must not get to pick where its own issue is triaged.
func TestAModelCannotChooseItsOwnLabels(t *testing.T) {
	t.Parallel()
	policy := issueLabelPolicy{Tracker: "forgejo", SandboxID: 353, DestinationID: 358}
	got := policy.withHarnessLabels(map[string]any{
		"title":         "a report",
		issueLabelField: []int{355, 999},
	})
	if !reflect.DeepEqual(got[issueLabelField], []int{353, 358}) {
		t.Errorf("labels = %v, want the harness's own ids only", got[issueLabelField])
	}
}

// A typo disables the control rather than mislabelling with a wrong id.
func TestAnUnreadableLabelIDDisablesIt(t *testing.T) {
	t.Parallel()
	for _, raw := range []string{"", "  ", "abc", "0", "-3", "3.5"} {
		if got := positiveInt(raw); got != 0 {
			t.Errorf("positiveInt(%q) = %d, want 0", raw, got)
		}
	}
	if got := positiveInt(" 353 "); got != 353 {
		t.Errorf("positiveInt(\" 353 \") = %d", got)
	}
}
