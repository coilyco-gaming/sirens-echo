package community

import (
	"bytes"
	"compress/gzip"
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"
	"time"
)

// The acceptance on sirens-echo#810: a log row carries service.name matching
// the trace side. See docs/sirens-echo-log-export.md.

// collectorCapture records what the exporters actually sent, by path.
type collectorCapture struct {
	mu     sync.Mutex
	bodies map[string][][]byte
}

func (c *collectorCapture) record(path string, body []byte) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.bodies[path] = append(c.bodies[path], body)
}

func (c *collectorCapture) forPath(path string) [][]byte {
	c.mu.Lock()
	defer c.mu.Unlock()
	return append([][]byte{}, c.bodies[path]...)
}

// fakeCollector accepts OTLP/HTTP on every signal path and keeps the bodies.
func fakeCollector(t *testing.T) (*httptest.Server, *collectorCapture) {
	t.Helper()
	capture := &collectorCapture{bodies: map[string][][]byte{}}
	server := httptest.NewServer(http.HandlerFunc(func(
		writer http.ResponseWriter,
		request *http.Request,
	) {
		body, err := io.ReadAll(request.Body)
		if err != nil {
			t.Errorf("read %s body: %v", request.URL.Path, err)
		}
		if request.Header.Get("Content-Encoding") == "gzip" {
			reader, err := gzip.NewReader(bytes.NewReader(body))
			if err != nil {
				t.Errorf("gunzip %s body: %v", request.URL.Path, err)
			} else {
				body, _ = io.ReadAll(reader)
			}
		}
		capture.record(request.URL.Path, body)
		writer.WriteHeader(http.StatusOK)
	}))
	return server, capture
}

// Protobuf stores strings literally, so containment reads the wire without
// decoding a schema this package does not otherwise use.
func TestALoggedLineIsExportedWithItsServiceName(t *testing.T) {
	server, capture := fakeCollector(t)
	defer server.Close()

	telemetry, err := NewTelemetry(context.Background(), Config{
		Definition:   Definition{AuditRole: "community", Identity: "Sirens Deep"},
		InstanceName: "sirens-deep",
		OTLPEndpoint: server.URL,
		LogWriter:    io.Discard,
	})
	if err != nil {
		t.Fatalf("NewTelemetry: %v", err)
	}
	telemetry.Info(context.Background(), "turn.completed")

	// Close flushes the batch processor, so the export is not a race.
	shutdown, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if err := telemetry.Close(shutdown); err != nil {
		t.Fatalf("Close: %v", err)
	}

	bodies := capture.forPath("/v1/logs")
	if len(bodies) == 0 {
		t.Fatal("no log export reached the collector, so the lane is still traces and metrics only")
	}
	var sawService, sawMessage bool
	for _, body := range bodies {
		if bytes.Contains(body, []byte("service.name")) &&
			bytes.Contains(body, []byte("sirens-deep")) {
			sawService = true
		}
		if bytes.Contains(body, []byte("turn.completed")) {
			sawMessage = true
		}
	}
	if !sawService {
		t.Error("the exported log carries no service.name, which is the key every " +
			"SigNoZ query reaches for")
	}
	if !sawMessage {
		t.Error("the exported log does not carry the line that was logged")
	}
}

// Traces and logs must key on the same field with the same value, or a
// dashboard cannot use one field across both signals.
func TestLogsAndTracesShareOneServiceName(t *testing.T) {
	server, capture := fakeCollector(t)
	defer server.Close()

	telemetry, err := NewTelemetry(context.Background(), Config{
		Definition:   Definition{AuditRole: "community", Identity: "Sirens Deep"},
		InstanceName: "sirens-deep",
		OTLPEndpoint: server.URL,
		LogWriter:    io.Discard,
	})
	if err != nil {
		t.Fatalf("NewTelemetry: %v", err)
	}
	ctx, span := telemetry.StartSpan(context.Background(), "community.turn")
	telemetry.Info(ctx, "turn.completed")
	span.End()

	shutdown, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if err := telemetry.Close(shutdown); err != nil {
		t.Fatalf("Close: %v", err)
	}

	for _, signal := range []string{"/v1/logs", "/v1/traces"} {
		bodies := capture.forPath(signal)
		if len(bodies) == 0 {
			t.Fatalf("%s received nothing", signal)
		}
		var named bool
		for _, body := range bodies {
			if bytes.Contains(body, []byte("sirens-deep")) {
				named = true
			}
		}
		if !named {
			t.Errorf("%s carries no sirens-deep, so the two signals key differently", signal)
		}
	}
}
