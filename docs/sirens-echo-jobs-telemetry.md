# Job-scoped telemetry

Service-scoped telemetry answers "how is the service doing". Once work is
durable the question that matters is "what happened in that run", and that needs
a different key.

## The key is the job id

Traces carry `service.name = sirens-deep`. Logs carry
`k8s.namespace.name = sirens-deep`. Neither is addressable by unit of work.

The job id rides the context, so it reaches both without every call site
threading it:

* **Spans** carry `sirens_echo.job.id`. Not only the root span. `StartSpan` adds
  the attribute whenever the context is inside a job, so `model.chat`,
  `mcp.tool.call`, and everything else under a job are queryable by id directly
  rather than only by walking down from the root.
* **Logs** carry `job_id` as a row field, beside the `trace_id` and `span_id`
  the logger already promotes. The correlation path already worked; the job id
  joins it.

A span or a row outside a job carries no job id at all. An attribute that is
sometimes absent is more useful than one that is sometimes wrong.

## Retrieval

Given a job id and nothing else:

* every span of that run, by the span attribute
* every log row of that run, by the row field
* the record itself, by `GET /v1/jobs/{id}`

Service and namespace are not needed for any of the three.

## The counter

`sirens_echo.jobs` counts a job reaching a state, labelled by `kind` and
`state`. Both are closed sets declared in this repository, so neither is an
unbounded label. There is no job id on the metric, deliberately: an id is
unbounded cardinality and belongs on spans and logs, not on a counter.

## Progress

A long job reports while it runs, not only at the end.

Updates **edit one message** rather than posting a column of them. The first
update posts a line and the job remembers it; every later update edits that
line, and the terminal notice replaces it. A job that runs for an hour leaves
one message that ends by saying how it went.

Progress is **rate limited per job**, so a chatty executor cannot flood its
origin. Updates inside the window are dropped rather than queued.

Progress is **advisory**. Every update is logged regardless, but delivery is
best effort and an executor must never depend on it. The message id is held in
memory, so a restart posts a fresh line rather than editing the old one.

**Mention safety** applies with empty allowed mentions. It matters more here
than on a reply, because a job speaks without being asked again.

When a thread is bound to the job, updates go there instead of the channel, so
job chatter stays out of the conversation that started it. **No production path
binds one today**, so updates land in the channel. See
[sirens-echo#620](https://forgejo.coilysiren.me/coilyco-gaming/sirens-echo/issues/620).

See [the lifecycle](sirens-echo-jobs-lifecycle.md) and
[observability](sirens-echo-observability.md).
