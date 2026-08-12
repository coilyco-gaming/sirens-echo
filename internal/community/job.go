package community

import (
	"errors"
	"fmt"
	"sort"
	"strings"
	"time"
)

// A job is a unit of work that outlives the turn that created it. See
// docs/sirens-echo-jobs.md.

// JobState is the closed set of states a job can be in. It is a metric label,
// so it never carries an identifier.
type JobState string

const (
	// JobQueued is accepted and durable, not yet started.
	JobQueued JobState = "queued"
	// JobRunning is executing. The only state a crash can strand.
	JobRunning JobState = "running"
	// JobCancelling has been asked to stop and has not yet stopped.
	JobCancelling JobState = "cancelling"
	// JobSucceeded, JobFailed, and JobCancelled are terminal.
	JobSucceeded JobState = "succeeded"
	JobFailed    JobState = "failed"
	JobCancelled JobState = "cancelled"
)

// jobTransitions is the state machine. Absence is a rejected transition, not a
// silent overwrite.
var jobTransitions = map[JobState][]JobState{
	JobQueued:     {JobRunning, JobCancelling, JobCancelled, JobFailed},
	JobRunning:    {JobSucceeded, JobFailed, JobCancelling},
	JobCancelling: {JobCancelled, JobSucceeded, JobFailed},
	JobSucceeded:  {},
	JobFailed:     {},
	JobCancelled:  {},
}

// Terminal reports a state no transition leaves.
func (s JobState) Terminal() bool {
	return s == JobSucceeded || s == JobFailed || s == JobCancelled
}

// Valid reports a state this harness knows.
func (s JobState) Valid() bool {
	_, known := jobTransitions[s]
	return known
}

// CanTransitionTo reports whether the machine allows the move. A transition to
// the same state is allowed and is a no-op, which keeps a retry idempotent.
func (s JobState) CanTransitionTo(next JobState) bool {
	if s == next {
		return true
	}
	for _, allowed := range jobTransitions[s] {
		if allowed == next {
			return true
		}
	}
	return false
}

// JobStates lists every state in a stable order, for documentation and metrics.
func JobStates() []JobState {
	states := make([]JobState, 0, len(jobTransitions))
	for state := range jobTransitions {
		states = append(states, state)
	}
	sort.Slice(states, func(i, j int) bool { return states[i] < states[j] })
	return states
}

// JobOrigin is where a job was asked for, so completion can be reported in the
// place the requester used.
type JobOrigin struct {
	// Transport is discord, http, or mcp.
	Transport string `json:"transport"`
	// ChannelID and MessageID are set for Discord. RequestID is set for the
	// synchronous transports, which have no durable place to reply to.
	ChannelID string `json:"channel_id,omitempty"`
	MessageID string `json:"message_id,omitempty"`
	RequestID string `json:"request_id,omitempty"`
	// ThreadID binds follow-up turns to this job when one exists.
	ThreadID string `json:"thread_id,omitempty"`
}

// Job is the durable record. Every field is deployment-safe: no prompt, no
// reply body, and no member text.
type Job struct {
	ID string `json:"id"`
	// IdempotencyKey collapses a redelivered submission onto one job.
	IdempotencyKey string `json:"idempotency_key"`
	// Principal is the account that asked, carried from the start because
	// adding an owner later is worse. It grants nothing. See docs/sirens-echo-jobs.md.
	Principal string `json:"principal"`
	// Kind names what the job does, from a closed set the harness declares.
	Kind      string    `json:"kind"`
	Origin    JobOrigin `json:"origin"`
	State     JobState  `json:"state"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
	// StartedAt and EndedAt are zero until the job reaches those points.
	StartedAt time.Time `json:"started_at,omitempty"`
	EndedAt   time.Time `json:"ended_at,omitempty"`
	// Outcome is a short harness-authored phrase, never upstream error text.
	Outcome string `json:"outcome,omitempty"`
	// Attempts counts executions started, so a restart cannot look like a
	// first run. See docs/sirens-echo-jobs.md.
	Attempts int `json:"attempts"`
	// Effects records what the job has already applied, so a resumed job does
	// not double-apply. Keys are step names the job kind declares.
	Effects map[string]string `json:"effects,omitempty"`
}

// validJobID keeps an id safe as a filename and as a metric-free log field.
func validJobID(value string) bool {
	if len(value) < 8 || len(value) > 64 {
		return false
	}
	for _, current := range value {
		switch {
		case current >= 'a' && current <= 'z':
		case current >= '0' && current <= '9':
		case current == '-':
		default:
			return false
		}
	}
	return true
}

// Validate rejects a record the store must never hold. A malformed job is a
// programming error, and persisting one hides it until a restart.
func (j Job) Validate() error {
	if !validJobID(j.ID) {
		return fmt.Errorf("job id must be 8 to 64 lowercase, digit, or dash characters")
	}
	if strings.TrimSpace(j.IdempotencyKey) == "" {
		return fmt.Errorf("job %s has no idempotency key", j.ID)
	}
	if strings.TrimSpace(j.Principal) == "" {
		return fmt.Errorf("job %s has no requesting principal", j.ID)
	}
	if strings.TrimSpace(j.Kind) == "" {
		return fmt.Errorf("job %s has no kind", j.ID)
	}
	if !j.State.Valid() {
		return fmt.Errorf("job %s has unknown state %q", j.ID, j.State)
	}
	switch j.Origin.Transport {
	case transportDiscord, transportHTTP, transportMCP:
	default:
		return fmt.Errorf("job %s has unknown transport %q", j.ID, j.Origin.Transport)
	}
	if j.Origin.Transport == transportDiscord && j.Origin.ChannelID == "" {
		return fmt.Errorf("job %s came from Discord with no channel to answer in", j.ID)
	}
	if j.CreatedAt.IsZero() {
		return fmt.Errorf("job %s has no creation time", j.ID)
	}
	return nil
}

// Notifiable reports an origin the runtime can deliver a result to after the
// requesting turn has gone. Only Discord has one.
func (j Job) Notifiable() bool {
	return j.Origin.Transport == transportDiscord && j.Origin.ChannelID != ""
}

// jobTransitionError names a refused move. The caller decides whether that is
// a conflict to report or a race to absorb.
type jobTransitionError struct {
	ID   string
	From JobState
	To   JobState
}

func (e jobTransitionError) Error() string {
	return fmt.Sprintf("job %s cannot move from %s to %s", e.ID, e.From, e.To)
}

// IsJobTransitionError reports a refused state change, which a caller racing
// another worker should treat as lost rather than as a fault.
func IsJobTransitionError(err error) bool {
	var refused jobTransitionError
	return errors.As(err, &refused)
}
