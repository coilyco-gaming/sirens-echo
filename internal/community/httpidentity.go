package community

import (
	"crypto/subtle"
	"net/http"
	"strings"
)

// A trusted caller on the tailnet, not on the internet. See
// docs/sirens-echo-http.md.

// httpTrustHeader carries the deployment's token. It is deliberately not
// X-Sirens-Caller, which is self-asserted and stays a rate-limit key.
const httpTrustHeader = "Authorization"

// httpTrustScheme is the only accepted form, so a bare token cannot be sent by
// accident and logged by something that expects a scheme.
const httpTrustScheme = "Bearer "

// callerTrusted reports whether a request presented the deployment's token.
// An empty configured token trusts nobody, which is the default.
func callerTrusted(request *http.Request, configured string) bool {
	if configured == "" || request == nil {
		return false
	}
	header := request.Header.Get(httpTrustHeader)
	if !strings.HasPrefix(header, httpTrustScheme) {
		return false
	}
	presented := strings.TrimSpace(strings.TrimPrefix(header, httpTrustScheme))
	// Constant time, because a check that leaks its answer through timing is
	// worse than no check: it looks like a control and is not one.
	return subtle.ConstantTimeCompare([]byte(presented), []byte(configured)) == 1
}
