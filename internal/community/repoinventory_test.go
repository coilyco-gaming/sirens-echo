package community

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

// The inventory is public-only because it carries no credential, not because it
// filters. That is the property worth testing. See sirens-echo#642.

const orgReposJSON = `[
 {"name":"sirens-echo","full_name":"coilyco-gaming/sirens-echo","description":"a harness",
  "html_url":"https://forge.example/coilyco-gaming/sirens-echo","language":"Go",
  "updated_at":"2026-08-13T10:00:00Z","private":false},
 {"name":"eco-app","full_name":"coilyco-gaming/eco-app","description":"",
  "html_url":"https://forge.example/coilyco-gaming/eco-app","language":"",
  "updated_at":"2026-08-12T09:00:00Z","private":false,"archived":true},
 {"name":"secrets","full_name":"coilyco-gaming/secrets","description":"nope",
  "html_url":"https://forge.example/coilyco-gaming/secrets","language":"Go",
  "updated_at":"2026-08-11T09:00:00Z","private":true}
]`

// inventoryAgainst serves the fixture and records what the tool sent.
func inventoryAgainst(t *testing.T, body string) (*repoInventorySession, *http.Header) {
	t.Helper()
	seen := &http.Header{}
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		*seen = r.Header.Clone()
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(body))
	}))
	t.Cleanup(server.Close)

	provider := &RepoInventoryProvider{
		BaseURL: server.URL, Org: "coilyco-gaming", Client: server.Client(),
	}
	session, err := provider.Open(context.Background())
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	return session.(*repoInventorySession), seen
}

// The guarantee. A credential is what would make a private repository visible,
// so the test is that none is sent rather than that the filter works.
func TestTheInventorySendsNoCredential(t *testing.T) {
	t.Parallel()
	session, seen := inventoryAgainst(t, orgReposJSON)
	if _, err := session.Call(context.Background(), repoInventoryToolName, nil); err != nil {
		t.Fatalf("call: %v", err)
	}
	for _, header := range []string{"Authorization", "Cookie", "X-Forgejo-Token"} {
		if value := seen.Get(header); value != "" {
			t.Errorf("the inventory sent %s: %q", header, value)
		}
	}
}

func TestTheInventoryListsPublicReposInOrder(t *testing.T) {
	t.Parallel()
	session, _ := inventoryAgainst(t, orgReposJSON)
	result, err := session.Call(context.Background(), repoInventoryToolName, nil)
	if err != nil {
		t.Fatalf("call: %v", err)
	}
	if result.IsError {
		t.Fatalf("the inventory failed: %s", result.Text)
	}
	for _, want := range []string{
		"2 public repositories in coilyco-gaming",
		"sirens-echo", "a harness", "language Go, updated 2026-08-13T10:00:00Z",
		"eco-app", "no description", "language unknown", ", archived",
	} {
		if !strings.Contains(result.Text, want) {
			t.Errorf("the listing is missing %q:\n%s", want, result.Text)
		}
	}
	// A private repository the endpoint should never have returned is still
	// dropped, because a wrong token elsewhere must not become a disclosure.
	if strings.Contains(result.Text, "secrets") {
		t.Errorf("a private repository reached the listing:\n%s", result.Text)
	}
}

// An unset org offers no tool at all, rather than a tool that returns nothing.
func TestAnUnconfiguredInventoryOffersNoTool(t *testing.T) {
	t.Parallel()
	for _, provider := range []*RepoInventoryProvider{
		{},
		{Org: "coilyco-gaming"},
		{BaseURL: "https://forge.example"},
	} {
		session, err := provider.Open(context.Background())
		if err != nil {
			t.Fatalf("open: %v", err)
		}
		if tools := session.Tools(); len(tools) != 0 {
			t.Errorf("an unconfigured inventory offered %d tools", len(tools))
		}
	}
}

// A forge that refuses says so, rather than reporting an empty organization,
// which would read as "this org has no public repositories".
func TestARefusedListingIsNotAnEmptyOrganization(t *testing.T) {
	t.Parallel()
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusNotFound)
	}))
	defer server.Close()
	provider := &RepoInventoryProvider{
		BaseURL: server.URL, Org: "absent", Client: server.Client(),
	}
	session, _ := provider.Open(context.Background())
	result, err := session.Call(context.Background(), repoInventoryToolName, nil)
	if err != nil {
		t.Fatalf("call: %v", err)
	}
	if !result.IsError {
		t.Errorf("a 404 was reported as a successful listing: %q", result.Text)
	}
}

// The tool description says public-only, because the model reads that and not
// this file when deciding what the answer covers.
func TestTheToolSaysItIsPublicOnly(t *testing.T) {
	t.Parallel()
	session, _ := inventoryAgainst(t, orgReposJSON)
	tools := session.Tools()
	if len(tools) != 2 {
		t.Fatalf("tools = %d, want the inventory and the file read", len(tools))
	}
	// Both, because a model reads the description and not this file when
	// deciding what an answer covers.
	for _, tool := range tools {
		if !strings.Contains(tool.Description, "Public only") {
			t.Errorf("%s does not bound what it covers: %q", tool.Name, tool.Description)
		}
	}
}

// The file read, sibling to the inventory. See sirens-echo#679.

// fileServer records the request and serves the body.
func fileServer(t *testing.T, body string, status int) (*repoInventorySession, *string, *http.Header) {
	t.Helper()
	path := new(string)
	seen := &http.Header{}
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		*path = r.URL.RequestURI()
		*seen = r.Header.Clone()
		if status != http.StatusOK {
			w.WriteHeader(status)
			return
		}
		_, _ = w.Write([]byte(body))
	}))
	t.Cleanup(server.Close)
	provider := &RepoInventoryProvider{
		BaseURL: server.URL, Org: "coilyco-gaming", Client: server.Client(),
	}
	session, _ := provider.Open(context.Background())
	return session.(*repoInventorySession), path, seen
}

func TestAPublicFileIsReturnedWithItsPath(t *testing.T) {
	t.Parallel()
	session, requested, seen := fileServer(t, "package community\n", http.StatusOK)
	result, err := session.Call(context.Background(), readFileToolName, map[string]any{
		"owner": "coilyco-gaming", "repo": "sirens-echo", "path": "internal/community/fetch.go",
	})
	if err != nil {
		t.Fatalf("call: %v", err)
	}
	if result.IsError {
		t.Fatalf("a public file was refused: %s", result.Text)
	}
	if !strings.Contains(result.Text, "package community") {
		t.Errorf("the body is missing:\n%s", result.Text)
	}
	if !strings.Contains(*requested, "/raw/internal/community/fetch.go") {
		t.Errorf("the path was not preserved through escaping: %s", *requested)
	}
	if value := seen.Get("Authorization"); value != "" {
		t.Errorf("the file read sent Authorization: %q", value)
	}
}

// A ref reaches the query rather than being dropped, or every read silently
// answers from the default branch.
func TestARefReachesTheRequest(t *testing.T) {
	t.Parallel()
	session, requested, _ := fileServer(t, "x", http.StatusOK)
	if _, err := session.Call(context.Background(), readFileToolName, map[string]any{
		"owner": "o", "repo": "r", "path": "a.go", "ref": "main",
	}); err != nil {
		t.Fatalf("call: %v", err)
	}
	if !strings.Contains(*requested, "ref=main") {
		t.Errorf("the ref was dropped: %s", *requested)
	}
}

// A path that climbs out is refused before any request is made.
func TestAnEscapingPathIsRefused(t *testing.T) {
	t.Parallel()
	for _, path := range []string{"../secrets", "a/../../b", "/etc/passwd"} {
		session, requested, _ := fileServer(t, "x", http.StatusOK)
		result, err := session.Call(context.Background(), readFileToolName, map[string]any{
			"owner": "o", "repo": "r", "path": path,
		})
		if err != nil {
			t.Fatalf("call: %v", err)
		}
		if !result.IsError {
			t.Errorf("%q was accepted", path)
		}
		if *requested != "" {
			t.Errorf("%q reached the forge as %s", path, *requested)
		}
	}
}

// A missing file says so rather than returning an empty body, which would read
// as a file that exists and is empty.
func TestAMissingFileIsAnError(t *testing.T) {
	t.Parallel()
	session, _, _ := fileServer(t, "", http.StatusNotFound)
	result, _ := session.Call(context.Background(), readFileToolName, map[string]any{
		"owner": "o", "repo": "r", "path": "absent.go",
	})
	if !result.IsError {
		t.Errorf("a 404 was reported as a file: %q", result.Text)
	}
}

// A file past the bound is cut with the loss stated, so a half file is not
// answered from with ordinary confidence.
func TestALargeFileIsTruncatedAudibly(t *testing.T) {
	t.Parallel()
	session, _, _ := fileServer(t, strings.Repeat("x", maxRepoFileBytes+500), http.StatusOK)
	result, _ := session.Call(context.Background(), readFileToolName, map[string]any{
		"owner": "o", "repo": "r", "path": "big.go",
	})
	if result.IsError {
		t.Fatalf("a large file was refused: %s", result.Text)
	}
	if !strings.Contains(result.Text, "truncated at") {
		t.Errorf("a truncated file does not say so:\n%s", result.Text[:200])
	}
}

// Missing arguments are refused rather than producing a request for an empty
// path, which the forge would answer with something unhelpful.
func TestTheFileReadRequiresItsArguments(t *testing.T) {
	t.Parallel()
	session, requested, _ := fileServer(t, "x", http.StatusOK)
	result, _ := session.Call(context.Background(), readFileToolName, map[string]any{
		"owner": "o", "repo": "r",
	})
	if !result.IsError || *requested != "" {
		t.Errorf("a missing path was not refused: %q, requested %q", result.Text, *requested)
	}
}
