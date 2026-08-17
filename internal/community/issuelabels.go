package community

import (
	"strconv"
	"strings"
)

// Every issue this service files carries the labels the harness applies, never
// ones the model supplied. See docs/sirens-echo-issues.md.

// issueLabelTool is the tracker verb the labels attach to. A different verb
// files nothing, so nothing else is touched.
const issueLabelTool = "create_issue"

// issueLabelField is the create-issue body field the tracker accepts.
const issueLabelField = "labels"

// positiveInt reads a label id. Anything unparsable or non-positive applies
// nothing, so a typo disables the control rather than mislabelling.
func positiveInt(raw string) int {
	value, err := strconv.Atoi(strings.TrimSpace(raw))
	if err != nil || value <= 0 {
		return 0
	}
	return value
}

// issueLabelPolicy decides which calls carry which labels. A zero id applies
// nothing, so each label is independently optional.
type issueLabelPolicy struct {
	Tracker string
	// SandboxID marks the contents unverified and is the control this file
	// exists for. See docs/sirens-echo-issues.md.
	SandboxID int
	// DestinationID is the move-to-repo label saying where the issue belongs.
	// The deployment sets the unknown one unless it knows better. See #756.
	DestinationID int
}

// labels returns the ids to send, in a fixed order so a request body does not
// vary between turns that meant the same thing.
func (p issueLabelPolicy) labels() []int {
	ids := make([]int, 0, 2)
	for _, id := range []int{p.SandboxID, p.DestinationID} {
		if id > 0 {
			ids = append(ids, id)
		}
	}
	return ids
}

// active reports whether the policy can do anything at all.
func (p issueLabelPolicy) active() bool {
	return p.Tracker != "" && len(p.labels()) > 0
}

// applies reports whether this call files an issue on the tracker.
func (p issueLabelPolicy) applies(tool registeredMCPTool) bool {
	return p.active() &&
		tool.serverName == p.Tracker &&
		tool.toolName == issueLabelTool
}

// withHarnessLabels returns the arguments the tracker should receive. It copies
// rather than mutating, so a retry sees what the model wrote.
func (p issueLabelPolicy) withHarnessLabels(arguments map[string]any) map[string]any {
	labelled := make(map[string]any, len(arguments)+1)
	for key, value := range arguments {
		labelled[key] = value
	}
	// Set rather than append. The model does not supply this field, and a
	// value it invented is not a reason to keep it.
	labelled[issueLabelField] = p.labels()
	return labelled
}
