package community

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"path/filepath"
	"sort"
	"strings"
)

// An inventory of the org's public repositories, read without a credential. See
// docs/sirens-echo-repo-inventory.md.

const (
	repoInventoryServer   = "repos"
	repoInventoryToolName = "list_public_repos"
)

// RepoInventoryProvider lists an organization's public repositories. An empty
// Org offers no tool at all, rather than a tool that returns nothing.
type RepoInventoryProvider struct {
	// BaseURL is the Forgejo root. Org names the organization to inventory.
	BaseURL string
	Org     string
	Client  *http.Client
}

// Open needs no per-turn state. The bound is the org, not the requester.
func (p *RepoInventoryProvider) Open(context.Context) (ToolSession, error) {
	if p == nil || strings.TrimSpace(p.Org) == "" || strings.TrimSpace(p.BaseURL) == "" {
		return &repoInventorySession{}, nil
	}
	client := p.Client
	if client == nil {
		// The fetch client, so an inventory cannot reach a private address
		// either. It carries no Authorization header and never will.
		client = newFetchClient()
	}
	return &repoInventorySession{
		baseURL: strings.TrimRight(strings.TrimSpace(p.BaseURL), "/"),
		org:     strings.TrimSpace(p.Org),
		client:  client,
	}, nil
}

type repoInventorySession struct {
	baseURL string
	org     string
	client  *http.Client
}

func (s *repoInventorySession) Close() error                   { return nil }
func (s *repoInventorySession) Grounding() []GroundingDocument { return nil }
func (s *repoInventorySession) Unavailable() []string          { return nil }

func (s *repoInventorySession) Tools() []ToolDefinition {
	if s.org == "" {
		return nil
	}
	return []ToolDefinition{s.readFileTool(), {
		Name:     repoInventoryToolName,
		Original: repoInventoryToolName,
		Server:   repoInventoryServer,
		Description: "List the public repositories in the " + s.org + " organization, " +
			"with description, language, and when each was last updated. " +
			"Public only: this reads without a credential, so a private " +
			"repository is not visible to it.",
		InputSchema: scratchObjectSchema(map[string]any{}, nil),
	}}
}

// repoRecord is the shape the inventory reports, matching the fields the
// existing skill emits so a reader of one can read the other.
type repoRecord struct {
	Name      string `json:"name"`
	Full      string `json:"full_name"`
	Desc      string `json:"description"`
	URL       string `json:"html_url"`
	Language  string `json:"language"`
	Updated   string `json:"updated_at"`
	Private   bool   `json:"private"`
	Internal  bool   `json:"internal"`
	Archived  bool   `json:"archived"`
	Templated bool   `json:"template"`
}

func (s *repoInventorySession) Call(
	ctx context.Context,
	name string,
	arguments map[string]any,
) (ToolResult, error) {
	if s.org == "" {
		return ToolResult{}, fmt.Errorf("model requested unavailable inventory tool %q", name)
	}
	if name == readFileToolName {
		return s.readPublicFile(ctx, arguments)
	}
	if name != repoInventoryToolName {
		return ToolResult{}, fmt.Errorf("model requested unavailable inventory tool %q", name)
	}
	records, err := s.list(ctx)
	if err != nil {
		return ToolResult{Text: err.Error(), IsError: true}, nil
	}
	if len(records) == 0 {
		return ToolResult{Text: "no public repositories in " + s.org}, nil
	}
	return ToolResult{Text: renderRepoInventory(s.org, records)}, nil
}

// list reads the org's repositories. No Authorization header is set anywhere in
// this file, which is what keeps a private repository out rather than a filter.
func (s *repoInventorySession) list(ctx context.Context) ([]repoRecord, error) {
	endpoint := s.baseURL + "/api/v1/orgs/" + url.PathEscape(s.org) + "/repos?limit=" +
		fmt.Sprint(maxRepoInventoryEntries)
	request, err := http.NewRequestWithContext(ctx, http.MethodGet, endpoint, nil)
	if err != nil {
		return nil, fmt.Errorf("that organization cannot be requested")
	}
	request.Header.Set("Accept", "application/json")
	response, err := s.client.Do(request)
	if err != nil {
		return nil, fmt.Errorf("the repository host did not answer")
	}
	defer func() { _ = response.Body.Close() }()
	if response.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("the repository host answered %d", response.StatusCode)
	}
	var records []repoRecord
	if err := json.NewDecoder(response.Body).Decode(&records); err != nil {
		return nil, fmt.Errorf("that response could not be read")
	}
	return publicRepos(records), nil
}

// publicRepos drops anything the endpoint should not have returned. Belt and
// braces: an unauthenticated read already cannot see these.
func publicRepos(records []repoRecord) []repoRecord {
	public := make([]repoRecord, 0, len(records))
	for _, record := range records {
		if record.Private || record.Internal {
			continue
		}
		public = append(public, record)
	}
	sort.Slice(public, func(i, j int) bool { return public[i].Name < public[j].Name })
	return public
}

// renderRepoInventory writes one line per repository. A missing field renders
// as a dash rather than an empty column, so a row cannot look truncated.
func renderRepoInventory(org string, records []repoRecord) string {
	var out strings.Builder
	fmt.Fprintf(&out, "%d public repositories in %s\n", len(records), org)
	for _, record := range records {
		fmt.Fprintf(&out, "\n%s\n  %s\n  %s\n  language %s, updated %s",
			record.Name,
			valueOrDefault(record.Desc, "no description"),
			valueOrDefault(record.URL, "-"),
			valueOrDefault(record.Language, "unknown"),
			valueOrDefault(record.Updated, "unknown"),
		)
		if record.Archived {
			out.WriteString(", archived")
		}
		out.WriteString("\n")
	}
	return out.String()
}

// readFileToolName reads one file from a public repository. Sibling to the
// inventory: same forge, same absence of a credential. See sirens-echo#679.
const readFileToolName = "read_public_file"

func (s *repoInventorySession) readFileTool() ToolDefinition {
	return ToolDefinition{
		Name:     readFileToolName,
		Original: readFileToolName,
		Server:   repoInventoryServer,
		Description: "Read one file from a public repository on this forge. " +
			"Public only: this reads without a credential, so a private " +
			"repository is not visible to it.",
		InputSchema: scratchObjectSchema(map[string]any{
			"owner": scratchStringProperty("Organization or user, such as " + repoInventoryOwnerHint + "."),
			"repo":  scratchStringProperty("Repository name."),
			"path":  scratchStringProperty("Path within the repository."),
			"ref":   scratchStringProperty("Branch, tag, or commit. Defaults to the default branch."),
		}, []string{"owner", "repo", "path"}),
	}
}

// repoInventoryOwnerHint keeps the schema's example from naming a real org this
// deployment may not serve.
const repoInventoryOwnerHint = "an organization"

// readPublicFile fetches one file. The path is refused rather than cleaned,
// because a caller that meant to escape should be told, not corrected.
func (s *repoInventorySession) readPublicFile(
	ctx context.Context,
	arguments map[string]any,
) (ToolResult, error) {
	owner := strings.TrimSpace(scratchStringArg(arguments, "owner"))
	repo := strings.TrimSpace(scratchStringArg(arguments, "repo"))
	path := strings.TrimSpace(scratchStringArg(arguments, "path"))
	if owner == "" || repo == "" || path == "" {
		return scratchRefusal("owner, repo and path are all required")
	}
	if escapesRepository(path) {
		return scratchRefusal("%s leaves the repository", path)
	}
	endpoint := s.baseURL + "/api/v1/repos/" + url.PathEscape(owner) + "/" +
		url.PathEscape(repo) + "/raw/" + escapeRepoPath(path)
	if ref := strings.TrimSpace(scratchStringArg(arguments, "ref")); ref != "" {
		endpoint += "?ref=" + url.QueryEscape(ref)
	}
	request, err := http.NewRequestWithContext(ctx, http.MethodGet, endpoint, nil)
	if err != nil {
		return scratchRefusal("that file cannot be requested")
	}
	response, err := s.client.Do(request)
	if err != nil {
		return scratchRefusal("the repository host did not answer")
	}
	defer func() { _ = response.Body.Close() }()
	if response.StatusCode != http.StatusOK {
		return scratchRefusal("the repository host answered %d for %s/%s/%s",
			response.StatusCode, owner, repo, path)
	}
	// One byte past the cap, so a file that fits is distinguishable from one
	// that does not. The same shape the fetch tool uses.
	body, err := io.ReadAll(io.LimitReader(response.Body, int64(maxRepoFileBytes)+1))
	if err != nil {
		return scratchRefusal("that file could not be read")
	}
	return ToolResult{Text: repoFileText(path, body)}, nil
}

// escapesRepository refuses a path that climbs out or is absolute. Checked
// before the request, so a refusal costs no round trip.
func escapesRepository(path string) bool {
	if strings.HasPrefix(path, "/") {
		return true
	}
	for _, segment := range strings.Split(filepath.ToSlash(path), "/") {
		if segment == ".." {
			return true
		}
	}
	return false
}

// escapeRepoPath escapes each segment, so a path keeps its separators while a
// segment carrying a slash or a space cannot forge one.
func escapeRepoPath(path string) string {
	segments := strings.Split(path, "/")
	for index, segment := range segments {
		segments[index] = url.PathEscape(segment)
	}
	return strings.Join(segments, "/")
}

func repoFileText(path string, body []byte) string {
	if len(body) <= maxRepoFileBytes {
		return fmt.Sprintf("%s\n%s", path, string(body))
	}
	kept := strings.ToValidUTF8(string(body[:maxRepoFileBytes]), "")
	return fmt.Sprintf("%s\n%s\n\n[truncated at %d bytes, this file is longer than that]",
		path, kept, maxRepoFileBytes)
}
