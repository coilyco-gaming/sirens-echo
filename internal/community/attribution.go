package community

import (
	"fmt"
	"sort"
	"strings"
)

// Attribution is evidence, not enforcement: it records who asked, and decides
// nothing. See docs/sirens-echo-attribution.md.

// EffectRecord is one external effect a job caused, resolved back to the
// principal that asked for it.
type EffectRecord struct {
	JobID     string
	Principal string
	Step      string
	Detail    string
	State     JobState
}

// AttributeEffects resolves a job's applied effects to their requester. The
// join is the record, so no user identifier has to live in telemetry.
func AttributeEffects(store JobStore, jobID string) ([]EffectRecord, error) {
	job, err := store.Get(jobID)
	if err != nil {
		return nil, err
	}
	steps := make([]string, 0, len(job.Effects))
	for step := range job.Effects {
		steps = append(steps, step)
	}
	sort.Strings(steps)
	records := make([]EffectRecord, 0, len(steps))
	for _, step := range steps {
		records = append(records, EffectRecord{
			JobID:     job.ID,
			Principal: job.Principal,
			Step:      step,
			Detail:    job.Effects[step],
			State:     job.State,
		})
	}
	return records, nil
}

// AttributeJob answers "who asked for this" from the record alone.
func AttributeJob(store JobStore, jobID string) (string, error) {
	job, err := store.Get(jobID)
	if err != nil {
		return "", err
	}
	if strings.TrimSpace(job.Principal) == "" {
		return "", fmt.Errorf("job %s carries no principal", jobID)
	}
	return job.Principal, nil
}
