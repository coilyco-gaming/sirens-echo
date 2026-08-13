package community

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

// Caller-visible guarantees of the turn surface, previously established only by
// probing the deployed pod. See docs/sirens-echo-http.md.

// testMaxContextMessages mirrors the value turnAgent pins, because the history
// boundary is asserted against it on both sides.
const testMaxContextMessages = 12

func httpTurnAgent(t *testing.T) *Agent {
	t.Helper()
	agent := turnAgent(Config{RateLimit: defaultRateLimitPolicy})
	// Each case posts its own body, so the reply cannot be keyed on the single
	// request id turnAgent's default client answers to.
	agent.completions = fixedCompletionClient{reply: "Echo is ready."}
	agent.ensureRuntimeDefaults()
	return agent
}

// postTurn sends one body to the turn path. Callers pass a distinct name so one
// case cannot spend another's per-user budget.
func postTurn(t *testing.T, agent *Agent, caller, body string) *httptest.ResponseRecorder {
	t.Helper()
	request := httptest.NewRequest(http.MethodPost, httpTurnPath, strings.NewReader(body))
	request.Header.Set("Content-Type", "application/json")
	request.Header.Set("X-Sirens-Caller", caller)
	recorder := httptest.NewRecorder()
	agent.HTTPHandler().ServeHTTP(recorder, request)
	return recorder
}

func TestHTTPTurnRejectsEveryMethodButPOST(t *testing.T) {
	t.Parallel()
	agent := httpTurnAgent(t)
	for _, method := range []string{http.MethodGet, http.MethodPut, http.MethodDelete, http.MethodPatch} {
		request := httptest.NewRequest(method, httpTurnPath, nil)
		recorder := httptest.NewRecorder()
		agent.HTTPHandler().ServeHTTP(recorder, request)
		if recorder.Code != http.StatusMethodNotAllowed {
			t.Errorf("%s status = %d, want 405", method, recorder.Code)
		}
		if allow := recorder.Header().Get("Allow"); allow != http.MethodPost {
			t.Errorf("%s Allow = %q, want POST", method, allow)
		}
	}
}

func TestHTTPTurnRejectsABodyThatIsNotJSON(t *testing.T) {
	t.Parallel()
	agent := httpTurnAgent(t)
	for name, body := range map[string]string{
		"prose":            "not json at all",
		"json array":       `["content"]`,
		"json string":      `"content"`,
		"truncated object": `{"content":`,
	} {
		recorder := postTurn(t, agent, "caller-"+name, body)
		if recorder.Code != http.StatusBadRequest {
			t.Errorf("%s status = %d, want 400", name, recorder.Code)
		}
		if got := recorder.Body.String(); !strings.Contains(got, "request body must be a JSON object") {
			t.Errorf("%s body = %q", name, got)
		}
	}
}

func TestHTTPTurnRequiresContentWhenNoPromptIsSelected(t *testing.T) {
	t.Parallel()
	agent := httpTurnAgent(t)
	for name, body := range map[string]string{
		"empty object":          `{}`,
		"empty content":         `{"content":""}`,
		"blank content":         `{"content":"   "}`,
		"newline content":       `{"content":"\n\t "}`,
		"author but no content": `{"author":"tester"}`,
	} {
		recorder := postTurn(t, agent, "caller-"+name, body)
		if recorder.Code != http.StatusBadRequest {
			t.Errorf("%s status = %d, want 400", name, recorder.Code)
		}
		if got := recorder.Body.String(); !strings.Contains(got, "content is required") {
			t.Errorf("%s body = %q", name, got)
		}
	}
}

// The caps count runes, so a multibyte author is not charged for bytes it did
// not spend. The accepting side is asserted too, because off-by-one is silent.
func TestHTTPTurnBoundsAuthorAndContentInRunes(t *testing.T) {
	t.Parallel()
	agent := httpTurnAgent(t)

	cases := []struct {
		name       string
		body       string
		wantStatus int
	}{
		{"author at the cap", fmt.Sprintf(`{"author":%q,"content":"hi"}`, strings.Repeat("é", 256)), http.StatusOK},
		{"author over the cap", fmt.Sprintf(`{"author":%q,"content":"hi"}`, strings.Repeat("é", 257)), http.StatusBadRequest},
		{"content at the cap", fmt.Sprintf(`{"content":%q}`, strings.Repeat("é", 16000)), http.StatusOK},
		{"content over the cap", fmt.Sprintf(`{"content":%q}`, strings.Repeat("é", 16001)), http.StatusBadRequest},
	}
	for _, testCase := range cases {
		recorder := postTurn(t, agent, "caller-"+testCase.name, testCase.body)
		if recorder.Code != testCase.wantStatus {
			t.Errorf("%s status = %d, want %d, body = %s",
				testCase.name, recorder.Code, testCase.wantStatus, recorder.Body.String())
		}
		if testCase.wantStatus != http.StatusBadRequest {
			continue
		}
		if got := recorder.Body.String(); !strings.Contains(got, "author or content is too long") {
			t.Errorf("%s body = %q", testCase.name, got)
		}
	}
}

// The limit is inclusive, so a caller filling the configured context exactly is
// admitted and only the message past it is refused.
func TestHTTPTurnEnforcesTheHistoryLimitOnBothSides(t *testing.T) {
	t.Parallel()
	agent := httpTurnAgent(t)

	history := func(count int) string {
		entries := make([]string, 0, count)
		for index := range count {
			entries = append(entries, fmt.Sprintf(`{"author":"member","content":"line %d"}`, index))
		}
		return fmt.Sprintf(`{"content":"hi","history":[%s]}`, strings.Join(entries, ","))
	}

	atLimit := postTurn(t, agent, "caller-at-limit", history(testMaxContextMessages))
	if atLimit.Code != http.StatusOK {
		t.Errorf("history at %d status = %d, want 200, body = %s",
			testMaxContextMessages, atLimit.Code, atLimit.Body.String())
	}

	overLimit := postTurn(t, agent, "caller-over-limit", history(testMaxContextMessages+1))
	if overLimit.Code != http.StatusBadRequest {
		t.Errorf("history at %d status = %d, want 400", testMaxContextMessages+1, overLimit.Code)
	}
	if got := overLimit.Body.String(); !strings.Contains(got, "history exceeds the configured context limit") {
		t.Errorf("over-limit body = %q", got)
	}
}

// Characterization. The body cap is enforced, but MaxBytesReader surfaces
// through the decoder, so a well-formed body is reported as malformed JSON.
func TestHTTPTurnRejectsABodyOverTheByteCap(t *testing.T) {
	t.Parallel()
	agent := httpTurnAgent(t)

	oversized := fmt.Sprintf(`{"content":%q}`, strings.Repeat("a", maxHTTPBody+1))
	recorder := postTurn(t, agent, "caller-oversized", oversized)
	if recorder.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want 400, body = %s", recorder.Code, recorder.Body.String())
	}
	if got := recorder.Body.String(); !strings.Contains(got, "request body must be a JSON object") {
		t.Errorf("oversized message = %q; a message naming the size cap means the "+
			"reporting defect was fixed and this assertion should follow it", got)
	}
}

// Characterization. Decoding is not strict, so a field from a newer client or a
// misspelled one is accepted in silence rather than refused.
func TestHTTPTurnAcceptsUnknownJSONFields(t *testing.T) {
	t.Parallel()
	agent := httpTurnAgent(t)

	recorder := postTurn(t, agent, "caller-unknown-field",
		`{"content":"hi","system_prompt":"override me","tools":["everything"]}`)
	if recorder.Code != http.StatusOK {
		t.Errorf("status = %d, want 200 for current behavior; a 400 means strict "+
			"decoding landed and this test should assert that, body = %s",
			recorder.Code, recorder.Body.String())
	}
}

func TestHTTPTurnRejectsAPromptMissingAServerOrName(t *testing.T) {
	t.Parallel()
	agent := httpTurnAgent(t)
	agent.tools = &MCPProvider{}
	t.Cleanup(func() { _ = agent.tools.Close() })

	for name, body := range map[string]string{
		"no server":    `{"prompt":{"server":"","name":"briefing"}}`,
		"no name":      `{"prompt":{"server":"eco","name":""}}`,
		"blank server": `{"prompt":{"server":"  ","name":"briefing"}}`,
		"neither":      `{"prompt":{}}`,
	} {
		recorder := postTurn(t, agent, "caller-"+name, body)
		if recorder.Code != http.StatusBadRequest {
			t.Errorf("%s status = %d, want 400", name, recorder.Code)
		}
		if got := recorder.Body.String(); !strings.Contains(got, "prompt requires a server and a name") {
			t.Errorf("%s body = %q", name, got)
		}
	}
}

// The unrostered-server error names what the caller supplied and nothing about
// where the roster would have reached. That distinction regresses silently.
func TestHTTPTurnRejectsAnUnrosteredPromptServerWithoutLeakingTheRoster(t *testing.T) {
	t.Parallel()
	agent := httpTurnAgent(t)
	agent.tools = &MCPProvider{Servers: []MCPServerDefinition{{Name: "eco"}}}
	t.Cleanup(func() { _ = agent.tools.Close() })

	recorder := postTurn(t, agent, "caller-unrostered", `{"prompt":{"server":"nope","name":"briefing"}}`)
	if recorder.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want 400, body = %s", recorder.Code, recorder.Body.String())
	}
	body := recorder.Body.String()
	if !strings.Contains(body, `MCP server "nope" is not in the roster`) {
		t.Errorf("body = %q", body)
	}
	for _, leak := range []string{"http://", "https://", "127.0.0.1", "localhost", ".svc"} {
		if strings.Contains(body, leak) {
			t.Errorf("caller-facing error leaks %q: %s", leak, body)
		}
	}
}

func TestHTTPUnknownPathIsNotFound(t *testing.T) {
	t.Parallel()
	agent := httpTurnAgent(t)
	for _, path := range []string{"/", "/v1", "/v1/turns", "/v1/turn/extra", "/admin"} {
		request := httptest.NewRequest(http.MethodPost, path, strings.NewReader(`{"content":"hi"}`))
		recorder := httptest.NewRecorder()
		agent.HTTPHandler().ServeHTTP(recorder, request)
		if recorder.Code != http.StatusNotFound {
			t.Errorf("%s status = %d, want 404", path, recorder.Code)
		}
	}
}

func TestHealthzRejectsEveryMethodButGET(t *testing.T) {
	t.Parallel()
	agent := httpTurnAgent(t)
	request := httptest.NewRequest(http.MethodPost, healthzPath, nil)
	recorder := httptest.NewRecorder()
	agent.HTTPHandler().ServeHTTP(recorder, request)
	if recorder.Code != http.StatusMethodNotAllowed {
		t.Errorf("status = %d, want 405", recorder.Code)
	}
	if allow := recorder.Header().Get("Allow"); allow != http.MethodGet {
		t.Errorf("Allow = %q, want GET", allow)
	}
}

// No shape of caller error reaches a 5xx, because a malformed request must not
// be counted against the service's own error rate.
func TestHTTPTurnNeverAnswersACallerErrorWithServerError(t *testing.T) {
	t.Parallel()
	agent := httpTurnAgent(t)
	agent.tools = &MCPProvider{}
	t.Cleanup(func() { _ = agent.tools.Close() })

	bodies := []string{
		"", "not json", `[]`, `{}`, `{"content":""}`, `{"content":null}`,
		`{"content":"hi","history":null}`, `{"content":"hi","history":"not a list"}`,
		`{"content":123}`, `{"author":123,"content":"hi"}`,
		`{"prompt":{}}`, `{"prompt":{"server":"nope","name":"x"}}`,
		fmt.Sprintf(`{"content":%q}`, strings.Repeat("é", 16001)),
	}
	for index, body := range bodies {
		recorder := postTurn(t, agent, fmt.Sprintf("caller-5xx-%d", index), body)
		if recorder.Code >= http.StatusInternalServerError {
			t.Errorf("body %d = %q produced %d", index, body, recorder.Code)
		}
	}
}

// A pending-cap denial carries Retry-After like every other class. It is the
// denial that dominates under burst, so it is the one a client most needs.
func TestQueueDenialCarriesRetryAfter(t *testing.T) {
	t.Parallel()
	limiter := newRateLimiter(RateLimitPolicy{MaxPending: 1, NotifyEvery: time.Minute}, 16)

	first := limiter.Admit(admissionRequest{UserKey: "http:one", ContextKey: transportHTTP, Queued: true})
	if first.Outcome != admissionAccepted {
		t.Fatalf("first outcome = %s, want accepted", first.Outcome)
	}
	second := limiter.Admit(admissionRequest{UserKey: "http:one", ContextKey: transportHTTP, Queued: true})
	if second.Outcome != admissionQueue {
		t.Fatalf("second outcome = %s, want %s", second.Outcome, admissionQueue)
	}
	if second.RetryAfter <= 0 {
		t.Error("queue denial carried no Retry-After, contradicting the documented contract")
	}
	if second.RetryAfter > time.Second {
		t.Errorf("RetryAfter = %s, over the one second shed window", second.RetryAfter)
	}
}

// Characterization. X-Sirens-Caller splits the per-user bucket and nothing else,
// so a second caller is denied on a tier its own budget never touched.
func TestCallerHeaderIsolatesTheUserTierOnly(t *testing.T) {
	t.Parallel()

	t.Run("the context bucket is shared", func(t *testing.T) {
		t.Parallel()
		limiter := newRateLimiter(RateLimitPolicy{
			PerUser:    RateLimit{Burst: 10, Every: time.Second},
			PerContext: RateLimit{Burst: 1, Every: time.Hour},
		}, 16)

		first := limiter.Admit(admissionRequest{UserKey: "http:one", ContextKey: transportHTTP})
		if first.Outcome != admissionAccepted {
			t.Fatalf("first outcome = %s, want accepted", first.Outcome)
		}
		second := limiter.Admit(admissionRequest{UserKey: "http:two", ContextKey: transportHTTP})
		if second.Outcome != admissionContext {
			t.Errorf("a distinct caller with an untouched budget got %s, want %s",
				second.Outcome, admissionContext)
		}
	})

	t.Run("the pending counter is shared", func(t *testing.T) {
		t.Parallel()
		limiter := newRateLimiter(RateLimitPolicy{MaxPending: 1}, 16)

		first := limiter.Admit(admissionRequest{UserKey: "http:one", ContextKey: transportHTTP, Queued: true})
		if first.Outcome != admissionAccepted {
			t.Fatalf("first outcome = %s, want accepted", first.Outcome)
		}
		second := limiter.Admit(admissionRequest{UserKey: "http:two", ContextKey: transportHTTP, Queued: true})
		if second.Outcome != admissionQueue {
			t.Errorf("a distinct caller got %s, want %s", second.Outcome, admissionQueue)
		}
	})
}

// An unidentified caller lands in one shared anonymous bucket rather than a
// fresh bucket per request.
func TestHTTPPrincipalDerivesTheLimiterKeyFromTheCallerHeader(t *testing.T) {
	t.Parallel()
	for name, testCase := range map[string]struct{ header, want string }{
		"absent": {"", "http:anonymous"},
		"blank":  {"   ", "http:anonymous"},
		"named":  {"fleet-client", "http:fleet-client"},
		"padded": {"  fleet-client  ", "http:fleet-client"},
	} {
		request := httptest.NewRequest(http.MethodPost, httpTurnPath, nil)
		if testCase.header != "" {
			request.Header.Set("X-Sirens-Caller", testCase.header)
		}
		if got := httpPrincipal(request); got != testCase.want {
			t.Errorf("%s principal = %q, want %q", name, got, testCase.want)
		}
	}
}
