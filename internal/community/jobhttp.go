package community

import (
	"encoding/json"
	"errors"
	"net/http"
	"strings"
)

// The HTTP surface is submit, poll, cancel. See docs/sirens-echo-jobs-lifecycle.md.

const jobsPath = "/v1/jobs/"

type jobSubmitRequest struct {
	Kind string `json:"kind"`
	// IdempotencyKey is optional. Absent derives one from the request id.
	IdempotencyKey string `json:"idempotency_key,omitempty"`
	RequestID      string `json:"request_id,omitempty"`
}

// jobView is the wire shape. It exposes the record's own fields and adds
// nothing, because the record is already free of member text.
type jobView struct {
	ID        string `json:"id"`
	Kind      string `json:"kind"`
	State     string `json:"state"`
	Outcome   string `json:"outcome,omitempty"`
	Attempts  int    `json:"attempts"`
	CreatedAt string `json:"created_at"`
	UpdatedAt string `json:"updated_at"`
}

func viewOfJob(job Job) jobView {
	return jobView{
		ID:        job.ID,
		Kind:      job.Kind,
		State:     string(job.State),
		Outcome:   job.Outcome,
		Attempts:  job.Attempts,
		CreatedAt: job.CreatedAt.UTC().Format("2006-01-02T15:04:05Z"),
		UpdatedAt: job.UpdatedAt.UTC().Format("2006-01-02T15:04:05Z"),
	}
}

// handleJobs routes the collection. Submission is the only POST here.
func (a *Agent) handleJobs(writer http.ResponseWriter, request *http.Request) {
	if a.jobs == nil {
		http.Error(writer, "jobs are not enabled", http.StatusNotFound)
		return
	}
	if request.Method != http.MethodPost {
		writer.Header().Set("Allow", http.MethodPost)
		http.Error(writer, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	request.Body = http.MaxBytesReader(writer, request.Body, int64(maxHTTPBody))
	var payload jobSubmitRequest
	if err := json.NewDecoder(request.Body).Decode(&payload); err != nil {
		if oversizeBody(err) {
			a.writeJobHTTPError(
				writer, request,
				http.StatusBadRequest, exceptionJobBodyTooLarge, oversizeBodyMessage,
			)
			return
		}
		a.writeJobHTTPError(
			writer, request,
			http.StatusBadRequest, exceptionJobRequestInvalid,
			"request body must be a JSON object",
		)
		return
	}
	principal := httpPrincipal(request)
	job, err := a.jobs.Submit(request.Context(), Submission{
		Kind:           payload.Kind,
		Principal:      principal,
		IdempotencyKey: payload.IdempotencyKey,
		Origin: JobOrigin{
			Transport: transportHTTP,
			RequestID: valueOrDefault(payload.RequestID, principal),
		},
	})
	if err != nil {
		a.writeJobError(writer, request, err)
		return
	}
	writeJSON(writer, http.StatusAccepted, viewOfJob(job))
}

// handleJob routes one job: read its state, or ask it to stop.
func (a *Agent) handleJob(writer http.ResponseWriter, request *http.Request) {
	if a.jobs == nil {
		http.Error(writer, "jobs are not enabled", http.StatusNotFound)
		return
	}
	rest := strings.TrimPrefix(request.URL.Path, jobsPath)
	id, action, _ := strings.Cut(rest, "/")
	if id == "" {
		http.Error(writer, "job id is required", http.StatusBadRequest)
		return
	}
	principal := httpPrincipal(request)
	switch {
	case action == "" && request.Method == http.MethodGet:
		job, err := a.jobs.Get(id, principal)
		if err != nil {
			a.writeJobError(writer, request, err)
			return
		}
		writeJSON(writer, http.StatusOK, viewOfJob(job))
	case action == "cancel" && request.Method == http.MethodPost:
		job, err := a.jobs.Cancel(request.Context(), id, principal)
		if err != nil {
			a.writeJobError(writer, request, err)
			return
		}
		writeJSON(writer, http.StatusOK, viewOfJob(job))
	default:
		http.Error(writer, "not found", http.StatusNotFound)
	}
}

// writeJobError maps a runner error onto a status without echoing internals.
// The response bodies are unchanged; only the record behind them is new.
func (a *Agent) writeJobError(
	writer http.ResponseWriter,
	request *http.Request,
	err error,
) {
	switch {
	case errors.Is(err, ErrJobNotFound), errors.Is(err, ErrNotJobOwner):
		// Another principal's job is indistinguishable from one that does not
		// exist, so an id cannot be probed for. One code keeps it that way.
		a.writeJobHTTPError(
			writer, request,
			http.StatusNotFound, exceptionJobNotFound, "job not found",
		)
	case IsGrantDenial(err):
		// A grant this deployment does not hold is not a request the caller can
		// correct, so it must not share 400 with a malformed one.
		a.writeJobHTTPError(
			writer, request,
			http.StatusForbidden, exceptionJobNotPermitted, "job kind is not permitted",
		)
	case errors.Is(err, ErrJobQueueFull):
		a.writeJobHTTPError(
			writer, request,
			http.StatusServiceUnavailable, exceptionJobQueueFull, "job queue is full",
		)
	default:
		a.writeJobHTTPError(
			writer, request,
			http.StatusBadRequest, exceptionJobRejected, "job could not be accepted",
		)
	}
}
