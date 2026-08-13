package community

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

// An upload is untrusted input the turn may read. Every case here is a way that
// could stop being true. See docs/sirens-echo-attachments.md.

// The host allowlist is the whole egress surface, so a payload naming any other
// address must not produce a request at all.
func TestOnlyADiscordCDNAddressIsFetched(t *testing.T) {
	t.Parallel()
	for _, refused := range []string{
		"https://evil.example.com/a.txt",
		"http://cdn.discordapp.com/a.txt",
		"https://cdn.discordapp.com.evil.example/a.txt",
		"https://169.254.169.254/latest/meta-data",
		"file:///etc/passwd",
		"",
	} {
		if permittedAttachmentURL(refused) {
			t.Errorf("%q was permitted", refused)
		}
	}
	if !permittedAttachmentURL("https://cdn.discordapp.com/attachments/1/2/notes.txt") {
		t.Error("a real Discord CDN address was refused")
	}
}

// The declared type and the extension belong to the uploader, so the bytes are
// what decides. A null byte is the cheapest strong binary signal.
func TestOnlyTextIsStored(t *testing.T) {
	t.Parallel()
	if textualAttachment([]byte("plain notes\nwith lines")) != true {
		t.Error("plain text was rejected")
	}
	if textualAttachment([]byte{0x89, 0x50, 0x4E, 0x47, 0x00, 0x01}) {
		t.Error("a binary signature was accepted")
	}
	if textualAttachment([]byte{0xff, 0xfe, 0xfd}) {
		t.Error("invalid UTF-8 was accepted")
	}
	if textualAttachment(nil) {
		t.Error("an empty body was accepted")
	}
}

// An oversized upload refuses rather than storing a partial document, since a
// truncated document read as whole is worse than no document.
func TestAnOversizedAttachmentRefuses(t *testing.T) {
	t.Parallel()
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte(strings.Repeat("a", maxAttachmentBytes+64)))
	}))
	defer server.Close()
	if _, err := fetchAttachment(context.Background(), server.Client(), server.URL); err == nil {
		t.Fatal("an oversized attachment was accepted")
	}
}

func TestAFailedFetchIsAnError(t *testing.T) {
	t.Parallel()
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusForbidden)
	}))
	defer server.Close()
	if _, err := fetchAttachment(context.Background(), server.Client(), server.URL); err == nil {
		t.Fatal("a 403 was accepted")
	}
}

// A turn with no scratchpad, no uploads, or no reserved writer must behave
// exactly as it does today rather than erroring.
func TestIngestIsInertWithoutASession(t *testing.T) {
	t.Parallel()
	ctx := WithAttachments(context.Background(), []AttachmentSource{
		{URL: "https://cdn.discordapp.com/attachments/1/2/a.txt"},
	})
	if stored := ingestAttachments(ctx, nil, nil); len(stored) != 0 {
		t.Errorf("a turn with no session stored %d files", len(stored))
	}
	if stored := ingestAttachments(context.Background(), nil, nil); len(stored) != 0 {
		t.Errorf("a turn with no uploads stored %d files", len(stored))
	}
}

// The model must not be able to write where an upload lands, or it could forge
// one and then cite it as something a member supplied.
func TestTheModelCannotWriteIntoTheUploadDirectory(t *testing.T) {
	t.Parallel()
	session := spillSession(t, "318190481467244544")
	writer, ok := session.(interface {
		Call(context.Context, string, map[string]any) (ToolResult, error)
	})
	if !ok {
		t.Fatal("the scratch session exposes no Call")
	}
	result, err := writer.Call(context.Background(), "scratch_write", map[string]any{
		"path": uploadPath(0), "content": "forged",
	})
	if err != nil {
		t.Fatalf("scratch_write: %v", err)
	}
	if !result.IsError {
		t.Error("the model wrote into the upload directory")
	}
}
