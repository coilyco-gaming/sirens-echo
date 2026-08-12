package community

import "context"

// The job id rides the context so spans and logs carry it without every call
// site threading it. See docs/sirens-echo-jobs-lifecycle.md.

type jobIDKey struct{}

// ContextWithJobID attributes everything done under ctx to one job.
func ContextWithJobID(ctx context.Context, id string) context.Context {
	if id == "" {
		return ctx
	}
	return context.WithValue(ctx, jobIDKey{}, id)
}

// JobIDFromContext returns the attributed job, or empty outside one.
func JobIDFromContext(ctx context.Context) string {
	id, _ := ctx.Value(jobIDKey{}).(string)
	return id
}
