// Package systemone calls Agent Proxy's /v1/systemone shim, which fronts
// TypeSafe's Jev decision model. See
// coilyco-flight-deck/agent-proxy/docs/systemone-shim.md for the upstream
// contract this client follows: one attempt, no client-side retry (the shim
// already retries 408/429/5xx and reads Retry-After), state required on every
// request, and the proxy's own 400/404/502/503/504 mapped onto typed errors
// here rather than treated as answers.
package systemone

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"
)

// QuestionType is the closed set of question shapes the upstream API accepts.
type QuestionType string

const (
	// TypeNoul is a yes/no question, answered with one probability.
	TypeNoul QuestionType = "noul"
	// TypeChoice picks among named options, each carrying a description.
	TypeChoice QuestionType = "choice"
	// TypeScore answers on a fixed scale, 2 to 10 levels.
	TypeScore QuestionType = "score"
)

// Criterion is one named option a choice question offers.
type Criterion struct {
	Name        string `json:"name"`
	Description string `json:"description"`
}

// Question is one typed question inside a request. Criteria is set only for
// TypeChoice, Levels only for TypeScore.
type Question struct {
	Key      string       `json:"key"`
	Type     QuestionType `json:"type"`
	Prompt   string       `json:"prompt"`
	Criteria []Criterion  `json:"criteria,omitempty"`
	Levels   int          `json:"levels,omitempty"`
}

// Request is one call to POST /v1/systemone. State is required upstream (a
// missing State is a 422), so a caller with nothing to say sends an empty object.
type Request struct {
	Model     string         `json:"model"`
	State     map[string]any `json:"state"`
	Questions []Question     `json:"questions"`
}

// Answer is what came back for one question. Probability is set for noul and
// choice (the chosen option's probability); Level is set for score.
type Answer struct {
	Key         string  `json:"key"`
	Option      string  `json:"option,omitempty"`
	Probability float64 `json:"probability,omitempty"`
	Level       int     `json:"level,omitempty"`
}

// Usage carries the token counts the reply reports. Output tokens are free
// per the upstream price sheet, so only input feeds cost.
type Usage struct {
	InputTokens  int `json:"input_tokens"`
	OutputTokens int `json:"output_tokens"`
}

// Response is one reply from POST /v1/systemone.
type Response struct {
	Model   string   `json:"model"`
	Answers []Answer `json:"answers"`
	Usage   Usage    `json:"usage"`
}

// AnswerByKey indexes the reply's answers, so a caller looks one up instead
// of scanning.
func (r Response) AnswerByKey() map[string]Answer {
	byKey := make(map[string]Answer, len(r.Answers))
	for _, answer := range r.Answers {
		byKey[answer.Key] = answer
	}
	return byKey
}

// FailureKind separates the shapes of failure the shim documents, so a
// caller can fall back on every one alike without inspecting an error string.
type FailureKind uint8

const (
	// FailureUnknown is any transport error not otherwise classified.
	FailureUnknown FailureKind = iota
	// FailureBadRequest is the proxy's own 400: an unreadable body.
	FailureBadRequest
	// FailureModelNotAllowed is the proxy's own 404: the model is not on
	// PROXY_SYSTEMONE_MODELS.
	FailureModelNotAllowed
	// FailureNoKeyMounted is the proxy's own 503: no key is mounted.
	FailureNoKeyMounted
	// FailureUpstreamUnreachable is the proxy's own 502.
	FailureUpstreamUnreachable
	// FailureTimeout is the proxy's own 504, or a local context deadline.
	FailureTimeout
	// FailureUpstreamRejected covers TypeSafe's own 401, 422, 429, 529
	// passed through unchanged.
	FailureUpstreamRejected
)

// Error wraps one failed call. Status is 0 for a failure detected locally
// (a marshal error, an enforced deadline) rather than one a server returned.
type Error struct {
	Kind    FailureKind
	Status  int
	Body    string
	Wrapped error
}

func (e *Error) Error() string {
	if e.Status != 0 {
		return fmt.Sprintf("systemone: status %d: %s", e.Status, e.Body)
	}
	return fmt.Sprintf("systemone: %v", e.Wrapped)
}

func (e *Error) Unwrap() error { return e.Wrapped }

func kindForStatus(status int) FailureKind {
	switch status {
	case http.StatusBadRequest:
		return FailureBadRequest
	case http.StatusNotFound:
		return FailureModelNotAllowed
	case http.StatusServiceUnavailable:
		return FailureNoKeyMounted
	case http.StatusBadGateway:
		return FailureUpstreamUnreachable
	case http.StatusGatewayTimeout:
		return FailureTimeout
	case http.StatusUnauthorized, http.StatusUnprocessableEntity,
		http.StatusTooManyRequests, 529:
		return FailureUpstreamRejected
	default:
		return FailureUnknown
	}
}

// Client calls one Agent Proxy deployment's /v1/systemone route.
type Client struct {
	// BaseURL is the Agent Proxy origin, e.g. "http://agent-proxy:8080".
	BaseURL string
	// Model is pinned by the caller's configuration, never chosen per call.
	Model string
	// Timeout bounds one attempt. The shim makes exactly one, so this is the
	// whole call's budget. Zero uses 10s, the shim's own default.
	Timeout time.Duration
	// HTTPClient carries the call. Nil takes http.DefaultClient.
	HTTPClient *http.Client
}

func (c Client) httpClient() *http.Client {
	if c.HTTPClient != nil {
		return c.HTTPClient
	}
	return http.DefaultClient
}

func (c Client) timeout() time.Duration {
	if c.Timeout > 0 {
		return c.Timeout
	}
	return 10 * time.Second
}

// Ask sends one request and returns the parsed reply. State defaults to an
// empty object when the caller supplies none, since the field is required.
func (c Client) Ask(ctx context.Context, req Request) (Response, error) {
	if strings.TrimSpace(c.BaseURL) == "" {
		return Response{}, &Error{Kind: FailureUnknown, Wrapped: fmt.Errorf("systemone: no BaseURL configured")}
	}
	if strings.TrimSpace(req.Model) == "" {
		req.Model = c.Model
	}
	if req.State == nil {
		req.State = map[string]any{}
	}

	callCtx, cancel := context.WithTimeout(ctx, c.timeout())
	defer cancel()

	body, err := json.Marshal(req)
	if err != nil {
		return Response{}, &Error{Kind: FailureUnknown, Wrapped: fmt.Errorf("marshal systemone request: %w", err)}
	}

	endpoint := strings.TrimRight(c.BaseURL, "/") + "/v1/systemone"
	httpReq, err := http.NewRequestWithContext(callCtx, http.MethodPost, endpoint, bytes.NewReader(body))
	if err != nil {
		return Response{}, &Error{Kind: FailureUnknown, Wrapped: fmt.Errorf("build systemone request: %w", err)}
	}
	httpReq.Header.Set("Content-Type", "application/json")

	resp, err := c.httpClient().Do(httpReq)
	if err != nil {
		if callCtx.Err() != nil {
			return Response{}, &Error{Kind: FailureTimeout, Wrapped: err}
		}
		return Response{}, &Error{Kind: FailureUnknown, Wrapped: err}
	}
	defer resp.Body.Close()

	raw, err := io.ReadAll(resp.Body)
	if err != nil {
		return Response{}, &Error{Kind: FailureUnknown, Wrapped: fmt.Errorf("read systemone response: %w", err)}
	}

	if resp.StatusCode != http.StatusOK {
		return Response{}, &Error{
			Kind:   kindForStatus(resp.StatusCode),
			Status: resp.StatusCode,
			Body:   strings.TrimSpace(string(raw)),
		}
	}

	var parsed Response
	if err := json.Unmarshal(raw, &parsed); err != nil {
		return Response{}, &Error{Kind: FailureUnknown, Wrapped: fmt.Errorf("decode systemone response: %w", err)}
	}
	return parsed, nil
}
