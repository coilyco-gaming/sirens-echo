package community

import (
	"strings"
	"testing"
	"unicode/utf8"
)

// An arbitrary-URL fetch runs inside the cluster, so the allowlist is the
// feature and the fetching is the easy part. See sirens-echo#412.

func fetchSessionFor(t *testing.T, hosts ...string) *fetchSession {
	t.Helper()
	session, err := (&FetchProvider{Hosts: hosts}).Open(t.Context())
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	return session.(*fetchSession)
}

// The property that makes this safe to land: no allowlist, no tool, so the
// prompt never mentions it and nothing can be talked into using it.
func TestNoAllowlistOffersNoTool(t *testing.T) {
	t.Parallel()
	session := fetchSessionFor(t)
	if tools := session.Tools(); len(tools) != 0 {
		t.Errorf("an unconfigured provider offered %d tools", len(tools))
	}
	if _, err := session.Call(t.Context(), fetchToolName, map[string]any{
		"url": "https://example.com",
	}); err == nil {
		t.Error("an unconfigured provider served a call")
	}
}

func TestAnAllowlistedHostIsAccepted(t *testing.T) {
	t.Parallel()
	session := fetchSessionFor(t, "eco-app.coilysiren.me")
	if _, err := session.allowedURL("https://eco-app.coilysiren.me/status"); err != nil {
		t.Errorf("an allowlisted host was refused: %v", err)
	}
	if tools := session.Tools(); len(tools) != 1 {
		t.Fatalf("offered %d tools, want 1", len(tools))
	}
	// The model reads the description, so it has to name the bound rather than
	// let the model discover it by being refused.
	if !strings.Contains(session.Tools()[0].Description, "eco-app.coilysiren.me") {
		t.Error("the tool description does not name the reachable hosts")
	}
}

// Every one of these is a way an allowlist gets defeated in practice.
func TestTheAllowlistRefusesEverythingElse(t *testing.T) {
	t.Parallel()
	session := fetchSessionFor(t, "eco-app.coilysiren.me")
	for name, raw := range map[string]string{
		"another host":       "https://evil.example/steal",
		"suffix lookalike":   "https://eco-app.coilysiren.me.evil.example/",
		"prefix lookalike":   "https://not-eco-app.coilysiren.me/",
		"plain http":         "http://eco-app.coilysiren.me/",
		"file scheme":        "file:///etc/passwd",
		"no scheme":          "eco-app.coilysiren.me",
		"cluster service":    "https://kubernetes.default.svc/api",
		"metadata address":   "https://169.254.169.254/latest/meta-data/",
		"loopback":           "https://127.0.0.1:8080/",
		"userinfo confusion": "https://eco-app.coilysiren.me@evil.example/",
	} {
		if _, err := session.allowedURL(raw); err == nil {
			t.Errorf("%s was allowed: %q", name, raw)
		}
	}
}

// The hostname check is not enough on its own: an allowlisted name can resolve
// to an internal address, deliberately or by accident.
func TestInternalAddressesAreRefusedAtDialTime(t *testing.T) {
	t.Parallel()
	for name, address := range map[string]string{
		"loopback":     "127.0.0.1:443",
		"private ten":  "10.1.2.3:443",
		"private 192":  "192.168.1.1:443",
		"private 172":  "172.16.5.4:443",
		"link local":   "169.254.169.254:443",
		"unspecified":  "0.0.0.0:443",
		"ipv6 loopack": "[::1]:443",
	} {
		if err := refusePrivateAddress(address); err == nil {
			t.Errorf("%s was dialled: %s", name, address)
		}
	}
	// A public address still works, or the tool does nothing at all.
	if err := refusePrivateAddress("93.184.216.34:443"); err != nil {
		t.Errorf("a public address was refused: %v", err)
	}
}

func TestTheAllowlistIsReadFromOneString(t *testing.T) {
	t.Parallel()
	hosts := fetchHosts(" eco-app.coilysiren.me , forgejo.coilysiren.me ,, ")
	if len(hosts) != 2 {
		t.Fatalf("parsed %d hosts, want 2: %v", len(hosts), hosts)
	}
	if len(fetchHosts("")) != 0 || len(fetchHosts("  ,  ")) != 0 {
		t.Error("an empty allowlist produced hosts")
	}
}

// The tailnet is carrier-grade NAT, which IsPrivate does not cover, so it read
// as public to every predicate in the guard. See sirens-echo#428.
func TestTheTailnetRangeIsRefused(t *testing.T) {
	t.Parallel()
	for _, address := range []string{
		"100.64.0.1:443",
		"100.100.100.100:443",
		"100.127.255.254:443",
	} {
		if err := refusePrivateAddress(address); err == nil {
			t.Errorf("%s was dialled, so the fetch tool reaches the tailnet", address)
		}
	}
}

// The ranges that were already covered stay covered, and a public address still
// dials. A guard that refuses everything is not a fix.
func TestTheGuardKeepsItsExistingReachAndRefusals(t *testing.T) {
	t.Parallel()
	for _, address := range []string{
		"10.0.0.1:443",
		"192.168.1.1:443",
		"172.16.0.1:443",
		"127.0.0.1:443",
		"169.254.169.254:443",
		"[::1]:443",
	} {
		if err := refusePrivateAddress(address); err == nil {
			t.Errorf("%s was dialled and should not have been", address)
		}
	}
	// 100.63 and 100.128 sit either side of the range, so an off-by-one in the
	// mask shows up here rather than as a silently widened block.
	for _, address := range []string{
		"93.184.216.34:443",
		"100.63.255.255:443",
		"100.128.0.0:443",
	} {
		if err := refusePrivateAddress(address); err != nil {
			t.Errorf("%s was refused, so the guard blocks public destinations: %v", address, err)
		}
	}
}

// A page cut at the cap used to come back looking whole, so the model answered
// from a document whose ending it never saw. See sirens-echo#435.
func TestATruncatedPageSaysSo(t *testing.T) {
	t.Parallel()
	oversized := []byte(strings.Repeat("a", maxFetchBytes+1))
	got := fetchText(200, oversized)
	if !strings.Contains(got, "truncated at") {
		t.Error("an oversize page came back with no sign it was cut")
	}
	// The body is still delivered. Refusing outright would waste a request that
	// succeeded, and a page is usually front-loaded.
	if !strings.Contains(got, "aaa") {
		t.Error("the page was discarded rather than truncated")
	}
}

func TestAPageThatFitsIsUnchanged(t *testing.T) {
	t.Parallel()
	got := fetchText(200, []byte("hello"))
	if got != "200\nhello" {
		t.Errorf("body = %q, want it untouched", got)
	}
	if strings.Contains(got, "truncated") {
		t.Error("a page within the cap was marked as truncated")
	}
	// Exactly at the cap is not over it.
	exact := fetchText(200, []byte(strings.Repeat("b", maxFetchBytes)))
	if strings.Contains(exact, "truncated") {
		t.Error("a page exactly at the cap was marked as truncated")
	}
}

// Cutting on a byte offset can split a rune, and a broken character at the seam
// is a different defect from the one being fixed.
func TestTruncationDoesNotSplitARune(t *testing.T) {
	t.Parallel()
	// Multi-byte characters arranged so the cap lands mid-rune.
	body := []byte(strings.Repeat("a", maxFetchBytes-1) + "日本語")
	got := fetchText(200, body)
	if !utf8.ValidString(got) {
		t.Error("truncation produced invalid UTF-8 at the seam")
	}
	if !strings.Contains(got, "truncated at") {
		t.Error("the oversize page was not marked")
	}
}
