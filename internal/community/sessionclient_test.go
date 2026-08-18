package community

import (
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

// A held-open session has no request boundary, so a whole-request deadline cuts
// it at exactly that deadline. See sirens-echo#160.

func TestTheSessionClientHasNoWholeRequestTimeout(t *testing.T) {
	t.Parallel()
	if got := sessionHTTPClient(telemetryOrNoop(nil)).Timeout; got != 0 {
		t.Errorf("session client carries a %v whole-request timeout, want none", got)
	}
}

// The observable failure, reproduced at millisecond scale: a client with a
// whole-request timeout cuts a response whose body is still open.
func TestAWholeRequestTimeoutCutsAHeldOpenBody(t *testing.T) {
	t.Parallel()
	release := make(chan struct{})
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
		if flusher, ok := w.(http.Flusher); ok {
			flusher.Flush()
		}
		<-release
	}))
	defer server.Close()
	defer close(release)

	bounded := &http.Client{Timeout: 50 * time.Millisecond}
	response, err := bounded.Get(server.URL)
	if err == nil {
		// Headers arrive first, so the status is a real 200 and the error lands
		// on the body read. That is the 200-with-an-error signature on #160.
		if response.StatusCode != http.StatusOK {
			t.Fatalf("status = %d, want 200", response.StatusCode)
		}
		buffer := make([]byte, 1)
		if _, readErr := response.Body.Read(buffer); readErr == nil {
			t.Fatal("a held-open body read succeeded under a whole-request timeout")
		}
		response.Body.Close()
	}
}

// The same server, through the session client, is not cut. The read blocks
// until the test releases it rather than until a deadline expires.
func TestTheSessionClientDoesNotCutAHeldOpenBody(t *testing.T) {
	t.Parallel()
	release := make(chan struct{})
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
		if flusher, ok := w.(http.Flusher); ok {
			flusher.Flush()
		}
		<-release
		_, _ = w.Write([]byte("x"))
	}))
	defer server.Close()

	response, err := sessionHTTPClient(telemetryOrNoop(nil)).Get(server.URL)
	if err != nil {
		t.Fatalf("session client refused a held-open response: %v", err)
	}
	defer response.Body.Close()
	read := make(chan error, 1)
	go func() {
		buffer := make([]byte, 1)
		_, readErr := response.Body.Read(buffer)
		read <- readErr
	}()
	// Staying open past a deadline is the whole property. What the body finally
	// yields is the server's business and races with its close.
	select {
	case readErr := <-read:
		t.Fatalf("the body was cut before the server wrote: %v", readErr)
	case <-time.After(200 * time.Millisecond):
	}
	close(release)
	<-read
}
