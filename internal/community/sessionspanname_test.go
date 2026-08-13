package community

import (
	"net/http"
	"strings"
	"testing"
)

// A held-open session's lifetime is not a request latency, and sharing a span
// name with the model path made it one. See sirens-echo#560.

func TestASessionSpanIsNotNamedLikeARequest(t *testing.T) {
	t.Parallel()
	for method, want := range map[string]string{
		http.MethodPost:   "mcp.session POST",
		http.MethodGet:    "mcp.session GET",
		http.MethodDelete: "mcp.session DELETE",
	} {
		request, err := http.NewRequest(method, "http://mcp.invalid/stream", nil)
		if err != nil {
			t.Fatalf("building a %s request: %v", method, err)
		}
		if got := mcpSessionSpanName("", request); got != want {
			t.Errorf("mcpSessionSpanName(%s) = %q, want %q", method, got, want)
		}
	}
}

// The whole point is that it cannot collide with the request-path name that
// otelhttp produces by default, which is the method alone.
func TestTheSessionNameCannotCollideWithTheRequestPath(t *testing.T) {
	t.Parallel()
	request, err := http.NewRequest(http.MethodPost, "http://mcp.invalid/stream", nil)
	if err != nil {
		t.Fatalf("building a request: %v", err)
	}
	name := mcpSessionSpanName("HTTP POST", request)
	if name == "HTTP POST" || name == http.MethodPost {
		t.Errorf("the session span still carries a request-path name: %q", name)
	}
	// Prefixed rather than replaced, so a reader still sees the verb and a
	// filter on the prefix catches every method at once.
	if !strings.HasPrefix(name, "mcp.session ") {
		t.Errorf("name %q does not share the mcp.session prefix", name)
	}
}

// The formatter must not read the operation string it is handed, because
// otelhttp passes a name this fix exists to discard.
func TestTheSuppliedOperationIsIgnored(t *testing.T) {
	t.Parallel()
	request, err := http.NewRequest(http.MethodPost, "http://mcp.invalid/stream", nil)
	if err != nil {
		t.Fatalf("building a request: %v", err)
	}
	for _, operation := range []string{"", "HTTP POST", "something else entirely"} {
		if got := mcpSessionSpanName(operation, request); got != "mcp.session POST" {
			t.Errorf("operation %q leaked into the name: %q", operation, got)
		}
	}
}
