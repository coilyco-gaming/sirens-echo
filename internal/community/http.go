package community

import (
	"context"
	"crypto/subtle"
	"encoding/json"
	"errors"
	"fmt"
	"math"
	"net/http"
	"strconv"
	"strings"
	"time"

	"go.opentelemetry.io/contrib/instrumentation/net/http/otelhttp"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/trace"
)

const (
	httpTurnPath = "/v1/turn"
	healthzPath  = "/healthz"
	maxHTTPBody  = 64 << 10
)

type httpTurnRequest struct {
	RequestID string            `json:"request_id"`
	Author    string            `json:"author"`
	Content   string            `json:"content"`
	History   []TranscriptEntry `json:"history"`
}

type httpTurnResponse struct {
	Reply string `json:"reply,omitempty"`
	Error string `json:"error,omitempty"`
}

// HTTPHandler exposes the same turn path as a summoned Discord message after
// Discord-specific gates pass. It is intended for private or loopback tests.
func (a *Agent) HTTPHandler() http.Handler {
	telemetry := telemetryOrNoop(a.telemetry)
	a.telemetry = telemetry
	a.ensureRuntimeDefaults()
	mux := http.NewServeMux()
	mux.HandleFunc(healthzPath, a.handleHealthz)
	mux.HandleFunc(readyzPath, a.handleReadyz)
	mux.Handle(httpTurnPath, instrumentHTTPRoute(telemetry, httpTurnPath, a.handleHTTPTurn))
	return mux
}

func instrumentHTTPRoute(
	telemetry *Telemetry,
	route string,
	handler http.HandlerFunc,
) http.Handler {
	routeHandler := http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		trace.SpanFromContext(request.Context()).SetAttributes(attribute.String("http.route", route))
		handler(writer, request)
	})
	return otelhttp.NewHandler(
		routeHandler,
		route,
		otelhttp.WithTracerProvider(telemetry.traceProvider),
		otelhttp.WithPropagators(telemetry.propagator),
		otelhttp.WithSpanNameFormatter(func(_ string, request *http.Request) string {
			return request.Method + " " + route
		}),
	)
}

func (a *Agent) handleHealthz(writer http.ResponseWriter, request *http.Request) {
	if request.Method != http.MethodGet {
		a.telemetry.RecordHealth(request.Context(), "healthz", "method_not_allowed")
		writer.Header().Set("Allow", http.MethodGet)
		http.Error(writer, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	a.telemetry.RecordHealth(request.Context(), "healthz", "ready")
	writeJSON(writer, http.StatusOK, map[string]bool{"ok": true})
}

func (a *Agent) handleReadyz(writer http.ResponseWriter, request *http.Request) {
	if request.Method != http.MethodGet {
		a.telemetry.RecordHealth(request.Context(), "readyz", "method_not_allowed")
		writer.Header().Set("Allow", http.MethodGet)
		http.Error(writer, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	started := time.Now()
	outcome := a.checkRouteReadiness(request.Context())
	a.telemetry.RecordHealth(request.Context(), "readyz", string(outcome))
	a.telemetry.RecordReadiness(request.Context(), outcome, time.Since(started))
	if outcome != readinessReady {
		writeJSON(writer, http.StatusServiceUnavailable, map[string]string{"status": "not_ready"})
		return
	}
	writeJSON(writer, http.StatusOK, map[string]string{"status": "ready"})
}

func (a *Agent) handleHTTPTurn(writer http.ResponseWriter, request *http.Request) {
	if request.Method != http.MethodPost {
		writer.Header().Set("Allow", http.MethodPost)
		a.writeHTTPError(
			writer,
			request,
			http.StatusMethodNotAllowed,
			exceptionHTTPTurnMethodNotAllowed,
			"method not allowed",
		)
		return
	}
	if !a.authorizedHTTPTurn(request) {
		writer.Header().Set("WWW-Authenticate", "Bearer")
		a.writeHTTPError(
			writer,
			request,
			http.StatusUnauthorized,
			exceptionHTTPTurnUnauthorized,
			"unauthorized",
		)
		return
	}
	request.Body = http.MaxBytesReader(writer, request.Body, maxHTTPBody)
	var payload httpTurnRequest
	decoder := json.NewDecoder(request.Body)
	if err := decoder.Decode(&payload); err != nil {
		a.writeHTTPError(
			writer,
			request,
			http.StatusBadRequest,
			exceptionHTTPTurnInvalidJSON,
			"request body must be a JSON object",
		)
		return
	}
	if strings.TrimSpace(payload.Content) == "" {
		a.writeHTTPError(
			writer,
			request,
			http.StatusBadRequest,
			exceptionHTTPTurnContentRequired,
			"content is required",
		)
		return
	}
	if payload.Author == "" {
		payload.Author = "manual test"
	}
	if len([]rune(payload.Author)) > 256 || len([]rune(payload.Content)) > 16000 {
		a.writeHTTPError(
			writer,
			request,
			http.StatusBadRequest,
			exceptionHTTPTurnInputTooLong,
			"author or content is too long",
		)
		return
	}
	if len(payload.History) > a.cfg.Definition.MaxContextMessages {
		a.writeHTTPError(
			writer,
			request,
			http.StatusBadRequest,
			exceptionHTTPTurnHistoryTooLong,
			"history exceeds the configured context limit",
		)
		return
	}
	if payload.RequestID == "" {
		payload.RequestID = fmt.Sprintf("http-%d", time.Now().UnixNano())
	}

	turn := &httpTurn{
		requestID: payload.RequestID,
		history:   append([]TranscriptEntry(nil), payload.History...),
		current: TranscriptEntry{
			Author:  payload.Author,
			Content: payload.Content,
		},
	}
	// HTTP shares the Discord admission policy, so a scripted client cannot
	// outspend the guilds it shares a deployment with.
	decision := a.limiter.Admit(admissionRequest{
		UserKey:    httpPrincipal(request),
		ContextKey: transportHTTP,
		Queued:     true,
	})
	if decision.Outcome.denied() {
		a.telemetry.RecordAdmission(request.Context(), string(decision.Outcome), transportHTTP)
		if decision.RetryAfter > 0 {
			writer.Header().Set(
				"Retry-After",
				strconv.Itoa(int(math.Ceil(decision.RetryAfter.Seconds()))),
			)
		}
		a.writeHTTPError(
			writer,
			request,
			http.StatusTooManyRequests,
			exceptionHTTPTurnRateLimited,
			"rate limit reached",
		)
		return
	}
	defer a.limiter.Release()
	a.telemetry.RecordAdmission(request.Context(), string(admissionAccepted), transportHTTP)

	if err := a.runSerialized(request.Context(), turn); err != nil {
		writeJSON(writer, http.StatusBadGateway, httpTurnResponse{
			Reply: turn.reply,
			Error: "turn failed",
		})
		return
	}
	writeJSON(writer, http.StatusOK, httpTurnResponse{Reply: turn.reply})
}

// authorizedHTTPTurn checks the deployment's shared secret in constant time.
// An empty configured token means loopback-only, which LoadConfig enforces.
func (a *Agent) authorizedHTTPTurn(request *http.Request) bool {
	if a.cfg.HTTPToken == "" {
		return true
	}
	supplied := strings.TrimSpace(
		strings.TrimPrefix(request.Header.Get("Authorization"), "Bearer "),
	)
	return subtle.ConstantTimeCompare([]byte(supplied), []byte(a.cfg.HTTPToken)) == 1
}

// httpPrincipal names the per-user limiter key for an HTTP caller. Callers that
// identify themselves get their own budget; the rest share one.
func httpPrincipal(request *http.Request) string {
	if caller := strings.TrimSpace(request.Header.Get("X-Sirens-Caller")); caller != "" {
		return "http:" + cleanTranscriptText(caller, 64)
	}
	return "http:anonymous"
}

func (a *Agent) writeHTTPError(
	writer http.ResponseWriter,
	request *http.Request,
	status int,
	code exceptionCode,
	message string,
) {
	a.telemetry.MarkSpanError(trace.SpanFromContext(request.Context()), code)
	http.Error(writer, message, status)
}

func writeJSON(writer http.ResponseWriter, status int, value any) {
	writer.Header().Set("Content-Type", "application/json")
	writer.WriteHeader(status)
	_ = json.NewEncoder(writer).Encode(value)
}

type httpTurn struct {
	requestID string
	history   []TranscriptEntry
	current   TranscriptEntry
	reply     string
}

func (t *httpTurn) RequestID() string { return t.requestID }

func (t *httpTurn) Transport() string { return transportHTTP }

func (t *httpTurn) Current() TranscriptEntry { return t.current }

func (t *httpTurn) History(context.Context) ([]TranscriptEntry, error) {
	return append([]TranscriptEntry(nil), t.history...), nil
}

func (t *httpTurn) Reply(_ context.Context, content string) error {
	if t.reply != "" {
		return errors.New("HTTP turn reply already set")
	}
	t.reply = content
	return nil
}
