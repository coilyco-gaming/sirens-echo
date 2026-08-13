package community

import (
	"strconv"
	"strings"
)

// Every issue this service files carries the sandbox label, applied by the
// harness rather than by the model. See docs/sirens-echo-sandbox-label.md.

// sandboxLabelTool is the tracker verb the label attaches to. A different verb
// files nothing, so nothing else is touched.
const sandboxLabelTool = "create_issue"

// sandboxLabelField is the create-issue body field the tracker accepts.
const sandboxLabelField = "labels"

// positiveInt reads a label id. Anything unparsable or non-positive applies
// nothing, so a typo disables the control rather than mislabelling.
func positiveInt(raw string) int {
	value, err := strconv.Atoi(strings.TrimSpace(raw))
	if err != nil || value <= 0 {
		return 0
	}
	return value
}

// sandboxLabelPolicy decides which calls carry the label. A zero LabelID
// applies nothing, which is the default and today's behaviour.
type sandboxLabelPolicy struct {
	Tracker string
	LabelID int
}

// active reports whether the policy can do anything at all.
func (p sandboxLabelPolicy) active() bool {
	return p.LabelID > 0 && p.Tracker != ""
}

// applies reports whether this call files an issue on the tracker.
func (p sandboxLabelPolicy) applies(tool registeredMCPTool) bool {
	return p.active() &&
		tool.serverName == p.Tracker &&
		tool.toolName == sandboxLabelTool
}

// withSandboxLabel returns the arguments the tracker should receive. It copies
// rather than mutating, so a retry sees what the model wrote.
func (p sandboxLabelPolicy) withSandboxLabel(arguments map[string]any) map[string]any {
	labelled := make(map[string]any, len(arguments)+1)
	for key, value := range arguments {
		labelled[key] = value
	}
	// Set rather than append. The model does not supply this field, and a
	// value it invented is not a reason to keep it.
	labelled[sandboxLabelField] = []int{p.LabelID}
	return labelled
}
