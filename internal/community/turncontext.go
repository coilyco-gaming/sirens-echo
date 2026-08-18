package community

import "context"

// The turn's request id rides the context so the tool mirror can key a
// trajectory on it. See docs/sirens-echo-tool-markup.md.

type requestIDKey struct{}

// ContextWithRequestID attributes everything done under ctx to one turn. On
// Discord the id is the summoning message's, which is the findable one.
func ContextWithRequestID(ctx context.Context, id string) context.Context {
	if id == "" {
		return ctx
	}
	return context.WithValue(ctx, requestIDKey{}, id)
}

// RequestIDFromContext returns the attributed turn, or empty outside one.
func RequestIDFromContext(ctx context.Context) string {
	id, _ := ctx.Value(requestIDKey{}).(string)
	return id
}
