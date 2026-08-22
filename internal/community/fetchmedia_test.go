package community

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

// fetchAgainst serves one response and returns what the tool made of it.
func fetchAgainst(t *testing.T, kind string, body []byte) ToolResult {
	t.Helper()
	server := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		if kind != "" {
			w.Header().Set("Content-Type", kind)
		}
		_, _ = w.Write(body)
	}))
	t.Cleanup(server.Close)

	host := strings.TrimPrefix(server.URL, "https://")
	host, _, _ = strings.Cut(host, ":")
	provider := &FetchProvider{Hosts: []string{host}, Client: server.Client()}
	session, err := provider.Open(context.Background())
	if err != nil {
		t.Fatalf("open fetch: %v", err)
	}
	result, err := session.Call(context.Background(), fetchToolName, map[string]any{
		"url": server.URL + "/asset",
	})
	if err != nil {
		t.Fatalf("fetch: %v", err)
	}
	return result
}

// A gif decoded as text is 32 KB of nothing, and what the caller needs is that
// it is a gif. See sirens-echo#1029.
func TestAMediaURLIsDescribedRatherThanDecoded(t *testing.T) {
	t.Parallel()
	gif := append([]byte("GIF89a"), make([]byte, 4096)...)
	result := fetchAgainst(t, "image/gif", gif)

	if result.IsError {
		t.Fatalf("a media url was refused: %s", result.Text)
	}
	if !strings.Contains(result.Text, "image/gif") {
		t.Errorf("result = %q, want the content type named", result.Text)
	}
	if !strings.Contains(result.Text, "not a page to read") {
		t.Errorf("result = %q, want it said this is not a document", result.Text)
	}
	if strings.Contains(result.Text, "GIF89a") {
		t.Errorf("result = %q, want the bytes left out", result.Text)
	}
}

// A page must still come back as a page, or this trades one broken answer for
// another.
func TestAPageIsStillReadWhole(t *testing.T) {
	t.Parallel()
	result := fetchAgainst(t, "text/html; charset=utf-8", []byte("<p>the joke, explained</p>"))

	if !strings.Contains(result.Text, "the joke, explained") {
		t.Errorf("result = %q, want the page body", result.Text)
	}
	if strings.Contains(result.Text, "not a page to read") {
		t.Errorf("result = %q, want a page treated as a page", result.Text)
	}
}

// A parameterised type and an unusual structured one are both text underneath,
// matched by suffix rather than by a list a new vendor type falls out of.
func TestTheReadableSetIsMatchedByShape(t *testing.T) {
	t.Parallel()
	for _, kind := range []string{
		"", "text/plain", "text/html; charset=utf-8", "application/json",
		"application/xml", "application/atom+xml", "application/vnd.api+json",
	} {
		if !fetchReadable(kind) {
			t.Errorf("%q is not read, so a readable page comes back as a description", kind)
		}
	}
	for _, kind := range []string{
		"image/gif", "image/png", "video/mp4", "application/octet-stream",
		"IMAGE/GIF", "image/webp; q=1",
	} {
		if fetchReadable(kind) {
			t.Errorf("%q is read, so its bytes reach the model as broken text", kind)
		}
	}
}

// A host that states no length still has to say what it is, or the caller
// learns nothing from the fetch it spent a round on.
func TestAMediaURLWithNoLengthStillNamesItsType(t *testing.T) {
	t.Parallel()
	rendered := fetchMedia(200, "image/gif", -1)
	if !strings.Contains(rendered, "image/gif") {
		t.Errorf("result = %q, want the type named", rendered)
	}
	if !strings.Contains(rendered, "unstated length") {
		t.Errorf("result = %q, want the missing length stated", rendered)
	}
}
