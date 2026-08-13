package community

import (
	"os"
	"strings"
	"testing"
)

// The label policy is covered. The line that invokes it was not, so deleting
// the call left every test green. See sirens-echo#208.

// A source check, because tool.session is a concrete *mcp.ClientSession and
// reaching Call needs a live server. The ordering is the property.
func TestTheSandboxLabelIsAppliedBeforeTheToolCall(t *testing.T) {
	t.Parallel()
	body, err := os.ReadFile("mcp.go")
	if err != nil {
		t.Fatalf("read mcp.go: %v", err)
	}
	source := string(body)
	apply := strings.Index(source, "s.sandbox.withSandboxLabel(arguments)")
	if apply < 0 {
		t.Fatal("nothing applies the sandbox label to a tool call's arguments, so " +
			"every issue this service files lands unlabelled and the policy is dead code")
	}
	dispatch := strings.Index(source, "tool.session.CallTool(")
	if dispatch < 0 {
		t.Fatal("the MCP dispatch moved, so this test no longer checks what it names")
	}
	// Applying after dispatch would leave a window where the issue exists
	// unlabelled, which is the reason the label is not a second call.
	if apply > dispatch {
		t.Error("the sandbox label is applied after the tool call rather than before, " +
			"so an unlabelled issue exists for the length of the request")
	}
	if strings.Count(source, "tool.session.CallTool(") != 1 {
		t.Error("a second MCP dispatch appeared, and this test only guards the first. " +
			"Route it through the labelled path or extend this check")
	}
}
