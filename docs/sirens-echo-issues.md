# Knowledge gaps and corrections

How Sirens Echo turns an unanswered question or a correction into a tracked
Forgejo issue. A definition without `issue_tracker` does none of this.

For a definition with `issue_tracker`, an unanswered question produces a
`knowledge-gap` draft and an explicit correction produces a `correction`
draft. These values affect only the issue title prefix. The runtime applies no
labels. A definition without `issue_tracker` must return `issue: null` and
state uncertainty in the reply.

The runtime removes Discord links, mention syntax, and long identifiers from
drafts. It requires a summary without member identity, handles, raw quotes,
secrets, or personal details. The automatic reporter calls the guarded
Forgejo MCP's HTTP tool projection to reuse an exact-title open issue or create
an ordinary issue. A reviewed change updates the local skill and regression
case.

## When the write fails

`forgejo.issue.failed` carries the failing MCP tool, the HTTP status, and
whether the tool reported the error rather than the transport. The remote
response body is discarded before the error is built, so it never reaches a
log. Without those fields a run of failures gives an operator no path to a
cause.

## See also

See [configuration](sirens-echo-config.md), [MCP tools](sirens-echo-tools.md),
and [the service](sirens-echo.md).
