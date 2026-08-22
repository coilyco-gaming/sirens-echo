package community

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

// Every other field on ProxyClient treats zero as the packaged default. This
// one treated it as a segfault on the first turn. See sirens-echo#959.
func TestAProxyClientWithNoHTTPClientStillCalls(t *testing.T) {
	t.Parallel()
	backend := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(
			`{"choices":[{"message":{"role":"assistant","content":"Echo is ready."}}]}`,
		))
	}))
	t.Cleanup(backend.Close)

	client := ProxyClient{BaseURL: backend.URL, Model: "test-model"}
	result, err := client.Complete(
		context.Background(),
		TurnPrompt{Message: "are you ready?"},
		"neutral model policy",
	)
	if err != nil {
		t.Fatalf("Complete with no HTTPClient: %v", err)
	}
	if !strings.Contains(result.Content, "ready") {
		t.Errorf("content = %q, want the backend's answer", result.Content)
	}
}

// A caller that does supply a client must still get theirs, or the default
// would quietly drop the instrumented transport NewAgent depends on.
func TestASuppliedHTTPClientIsStillUsed(t *testing.T) {
	t.Parallel()
	called := false
	backend := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		called = true
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(
			`{"choices":[{"message":{"role":"assistant","content":"Echo is ready."}}]}`,
		))
	}))
	t.Cleanup(backend.Close)

	supplied := &http.Client{}
	client := ProxyClient{BaseURL: backend.URL, Model: "test-model", HTTPClient: supplied}
	if got := client.httpClient(); got != supplied {
		t.Fatalf("httpClient() = %p, want the supplied client %p", got, supplied)
	}
	if _, err := client.Complete(
		context.Background(),
		TurnPrompt{Message: "are you ready?"},
		"neutral model policy",
	); err != nil {
		t.Fatalf("Complete: %v", err)
	}
	if !called {
		t.Error("the supplied client never reached the backend")
	}
}
