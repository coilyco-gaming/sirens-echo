package community

import (
	"reflect"
	"testing"
)

// The model never supplies this label and cannot omit it. See sirens-echo#208.

func sandboxPolicy() sandboxLabelPolicy {
	return sandboxLabelPolicy{Tracker: "forgejo", LabelID: 353}
}

// The default applies nothing, so landing this changes no deployment until one
// configures a label.
func TestNoConfiguredLabelAppliesNothing(t *testing.T) {
	t.Parallel()
	for name, policy := range map[string]sandboxLabelPolicy{
		"no label":   {Tracker: "forgejo"},
		"no tracker": {LabelID: 353},
		"neither":    {},
		"negative":   {Tracker: "forgejo", LabelID: -1},
	} {
		if policy.active() {
			t.Errorf("%s reported itself active", name)
		}
		if policy.applies(registeredMCPTool{serverName: "forgejo", toolName: sandboxLabelTool}) {
			t.Errorf("%s applied to a create-issue call", name)
		}
	}
}

func TestItAppliesOnlyToFilingOnTheTracker(t *testing.T) {
	t.Parallel()
	policy := sandboxPolicy()
	if !policy.applies(registeredMCPTool{serverName: "forgejo", toolName: sandboxLabelTool}) {
		t.Error("filing on the tracker was not labelled")
	}
	for name, tool := range map[string]registeredMCPTool{
		"another server": {serverName: "eco", toolName: sandboxLabelTool},
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
	got := sandboxPolicy().withSandboxLabel(map[string]any{
		"title":           "a report",
		"body":            "from a member",
		sandboxLabelField: []int{999},
	})
	if !reflect.DeepEqual(got[sandboxLabelField], []int{353}) {
		t.Errorf("labels = %v, want only the configured id", got[sandboxLabelField])
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
	sandboxPolicy().withSandboxLabel(original)
	if _, present := original[sandboxLabelField]; present {
		t.Error("the caller's arguments were mutated")
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
