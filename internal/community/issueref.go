package community

import (
	"encoding/json"
	"regexp"
	"strings"
)

// An issue reference outlives the conversation that wrote it, so the harness
// links what the model named in short form. See docs/sirens-echo-issues.md.

// issueURLPattern matches a canonical Forgejo issue URL. The trailing fragment
// is excluded, because the bare URL is the durable form.
var issueURLPattern = regexp.MustCompile(`https?://[^\s"'<>)\]]+/issues/(\d+)`)

// shortIssueRef matches the ambiguous form. The leading boundary rejects a
// channel mention, a URL fragment, and a number inside a longer reference.
var shortIssueRef = regexp.MustCompile(`(?:^|[^A-Za-z0-9_/#-])#(\d+)\b`)

// createIssueSuffix names the tool whose result reaches the member whether or
// not the model mentioned filing anything.
const createIssueSuffix = "__create_issue"

// referenceHeading labels the appended block. It carries no first person and no
// emoji, because it is appended after the response-style check has run.
const referenceHeading = "Referenced issues:"

// AppendIssueReferences links the issues a turn observed or filed. Every URL it
// appends came back from a tool call, so it states nothing unobserved.
func AppendIssueReferences(reply string, executed ...ExecutedTool) string {
	observed := observedIssueURLs(executed)
	if len(observed) == 0 {
		return reply
	}
	appended := make([]string, 0, len(observed))
	seen := make(map[string]struct{}, len(observed))
	add := func(url string) {
		if url == "" || strings.Contains(reply, url) {
			return
		}
		if _, done := seen[url]; done {
			return
		}
		seen[url] = struct{}{}
		appended = append(appended, url)
	}
	// A short form is the reported defect, so it resolves first and in the
	// order a reader meets it.
	for _, match := range shortIssueRef.FindAllStringSubmatch(reply, -1) {
		add(observed[match[1]])
	}
	// An issue this turn filed belongs in the reply even when the model said
	// nothing about it.
	for _, url := range createdIssueURLs(executed) {
		add(url)
	}
	return withReferenceBlock(reply, appended)
}

// withReferenceBlock renders the block within the send budget. A block that
// cannot fit whole is dropped rather than truncated into a broken URL.
func withReferenceBlock(reply string, urls []string) string {
	if len(urls) == 0 {
		return reply
	}
	trimmed := strings.TrimRight(reply, " \t\n")
	for len(urls) > 0 {
		block := "\n\n" + referenceHeading + "\n" + strings.Join(urls, "\n")
		if len([]rune(trimmed))+len([]rune(block)) <= discordReplyLimit {
			return trimmed + block
		}
		urls = urls[:len(urls)-1]
	}
	return reply
}

// observedIssueURLs maps an issue number to the web URL a tool returned for it.
// Scanning every result is deliberate: a quoted sibling issue is a real source.
func observedIssueURLs(executed []ExecutedTool) map[string]string {
	observed := make(map[string]string)
	for _, tool := range executed {
		for _, ref := range issueRefsIn(tool.Result) {
			previous, seen := observed[ref.number]
			if !seen {
				observed[ref.number] = ref.url
				continue
			}
			// One number, two repositories. Linking either is a guess, and the
			// empty value suppresses it. See docs/sirens-echo-issues.md.
			if previous != ref.url {
				observed[ref.number] = ""
			}
		}
	}
	return observed
}

// createdIssueURLs names what this turn filed, reading the response field
// rather than the payload, so a quoted issue is not mistaken for the new one.
func createdIssueURLs(executed []ExecutedTool) []string {
	urls := make([]string, 0)
	for _, tool := range executed {
		if !strings.HasSuffix(tool.Name, createIssueSuffix) {
			continue
		}
		if url := createdIssueURL(tool.Result); url != "" {
			urls = append(urls, url)
		}
	}
	return urls
}

func createdIssueURL(result string) string {
	var payload struct {
		Result struct {
			HTMLURL string `json:"html_url"`
		} `json:"result"`
	}
	if err := json.Unmarshal([]byte(result), &payload); err == nil {
		if url := issueURLPattern.FindString(payload.Result.HTMLURL); url != "" {
			return url
		}
	}
	// An unparsed result still names the issue, and the first web issue URL in
	// it beats linking nothing.
	for _, ref := range issueRefsIn(result) {
		return ref.url
	}
	return ""
}

type issueRef struct {
	number string
	url    string
}

func issueRefsIn(result string) []issueRef {
	matches := issueURLPattern.FindAllStringSubmatch(result, -1)
	refs := make([]issueRef, 0, len(matches))
	for _, match := range matches {
		// The API URL for the same issue rides along in the same payload and
		// is not a link a member can follow.
		if strings.Contains(match[0], "/api/") {
			continue
		}
		refs = append(refs, issueRef{number: match[1], url: match[0]})
	}
	return refs
}
