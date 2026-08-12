package community

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"strings"
)

// Submission is one request to do work. The harness turns it into a Job.
type Submission struct {
	// Kind names the work, from the closed set the harness declares.
	Kind string
	// Principal is the account that asked.
	Principal string
	Origin    JobOrigin
	// IdempotencyKey collapses a redelivery. Empty derives one from the
	// origin, which is what makes at-least-once transports safe by default.
	IdempotencyKey string
}

// JobKinds is the closed set of work this harness will accept. A kind is a
// capability, so widening it is a reviewed act rather than a caller's choice.
var JobKinds = map[string]string{
	"echo": "return the submitted request, for proving the lifecycle end to end",
}

// DeriveIdempotencyKey builds a key from the origin when a caller supplies
// none. Discord redelivers a message with the same id, so the message is the
// natural unit of "the same request".
func DeriveIdempotencyKey(submission Submission) string {
	if trimmed := strings.TrimSpace(submission.IdempotencyKey); trimmed != "" {
		return trimmed
	}
	parts := []string{
		submission.Kind,
		submission.Origin.Transport,
		submission.Origin.ChannelID,
		submission.Origin.MessageID,
		submission.Origin.RequestID,
	}
	return "origin:" + strings.Join(parts, "|")
}

// JobIDFor derives the id from the idempotency key, so a redelivery that races
// itself cannot produce two ids for one key even before the store dedups.
func JobIDFor(key string) string {
	sum := sha256.Sum256([]byte(key))
	return "job-" + hex.EncodeToString(sum[:10])
}

// PrepareJob validates a submission and returns the record to store. It does
// not touch the store, so a caller can reject before persisting anything.
func PrepareJob(submission Submission) (Job, error) {
	if _, known := JobKinds[submission.Kind]; !known {
		return Job{}, fmt.Errorf("unknown job kind %q", submission.Kind)
	}
	if strings.TrimSpace(submission.Principal) == "" {
		return Job{}, fmt.Errorf("job submission has no requesting principal")
	}
	key := DeriveIdempotencyKey(submission)
	job := Job{
		ID:             JobIDFor(key),
		IdempotencyKey: key,
		Principal:      submission.Principal,
		Kind:           submission.Kind,
		Origin:         submission.Origin,
		State:          JobQueued,
	}
	return job, nil
}

// RecordEffect marks a step applied, so a resumed job can skip what it already
// did. Recording is idempotent and never rewrites an existing value.
func RecordEffect(store JobStore, id, step, detail string) (Job, error) {
	job, err := store.Get(id)
	if err != nil {
		return Job{}, err
	}
	if _, applied := job.Effects[step]; applied {
		return job, nil
	}
	// A same-state transition is the machine's no-op, which is how an effect is
	// recorded without pretending the job moved.
	return store.Transition(id, job.State, func(target *Job) {
		if target.Effects == nil {
			target.Effects = make(map[string]string, 1)
		}
		target.Effects[step] = detail
	})
}

// EffectApplied reports whether a step already ran for this job.
func EffectApplied(job Job, step string) bool {
	_, applied := job.Effects[step]
	return applied
}
