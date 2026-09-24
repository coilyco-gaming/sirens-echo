package community

import "context"

// droppedServersKey carries route.jev's server verdict to the answer's
// Complete call only. See docs/sirens-echo-tools.md.
type droppedServersKey struct{}

func withDroppedServers(ctx context.Context, names []string) context.Context {
	if len(names) == 0 {
		return ctx
	}
	set := make(map[string]bool, len(names))
	for _, name := range names {
		set[name] = true
	}
	return context.WithValue(ctx, droppedServersKey{}, set)
}

// droppedServersFrom is nil, which drops nothing, when no verdict was attached.
func droppedServersFrom(ctx context.Context) map[string]bool {
	set, _ := ctx.Value(droppedServersKey{}).(map[string]bool)
	return set
}
