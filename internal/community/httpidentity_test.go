package community

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

// A trusted caller on the tailnet. Unset trusts nobody, which is what this
// endpoint did before tokens existed. See sirens-echo#165.

func requestWithAuth(header string) *http.Request {
	request := httptest.NewRequest(http.MethodPost, "/v1/turn", nil)
	if header != "" {
		request.Header.Set("Authorization", header)
	}
	return request
}

// The property that makes this safe to land: with no token configured, no
// request is trusted, however it is dressed.
func TestNoConfiguredTokenTrustsNobody(t *testing.T) {
	t.Parallel()
	for _, header := range []string{
		"",
		"Bearer ",
		"Bearer anything",
		"Bearer " + "",
	} {
		if callerTrusted(requestWithAuth(header), "") {
			t.Errorf("an unconfigured deployment trusted %q", header)
		}
	}
}

func TestTheConfiguredTokenIsTrusted(t *testing.T) {
	t.Parallel()
	if !callerTrusted(requestWithAuth("Bearer s3cret-token"), "s3cret-token") {
		t.Error("the configured token was not trusted")
	}
}

func TestEverythingElseIsNotTrusted(t *testing.T) {
	t.Parallel()
	cases := map[string]string{
		"no header":         "",
		"wrong token":       "Bearer wrong-token",
		"empty bearer":      "Bearer ",
		"missing scheme":    "s3cret-token",
		"wrong scheme":      "Basic s3cret-token",
		"prefix of token":   "Bearer s3cret",
		"token plus extra":  "Bearer s3cret-token-more",
		"scheme mismatched": "bearer s3cret-token",
	}
	for name, header := range cases {
		if callerTrusted(requestWithAuth(header), "s3cret-token") {
			t.Errorf("%s was trusted: %q", name, header)
		}
	}
}

// X-Sirens-Caller is self-asserted and stays a rate-limit key. If it ever
// grants trust, an unauthenticated caller becomes a principal by typing a name.
func TestTheSelfAssertedCallerHeaderGrantsNoTrust(t *testing.T) {
	t.Parallel()
	request := httptest.NewRequest(http.MethodPost, "/v1/turn", nil)
	request.Header.Set("X-Sirens-Caller", "coilysiren")
	if callerTrusted(request, "s3cret-token") {
		t.Error("the self-asserted caller header granted trust")
	}
	// It still does its own job, which this must not have broken.
	if httpPrincipal(request) == "http:anonymous" {
		t.Error("the caller header stopped partitioning the rate limit")
	}
}

func TestANilRequestIsNotTrusted(t *testing.T) {
	t.Parallel()
	if callerTrusted(nil, "s3cret-token") {
		t.Error("a nil request was trusted")
	}
}
