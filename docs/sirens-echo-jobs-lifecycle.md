# Job lifecycle

Submit returns an id. Polling reports state. Completion notifies the origin.
Cancellation stops the work and says so. See [the job record](sirens-echo-jobs.md)
for the state machine underneath.

## Submit

`POST /v1/jobs` with a kind, and optionally an idempotency key.

Submission returns `202` with the id and `queued` state rather than a finished
answer, so a request that will take minutes does not hold the transport open.
A redelivery returns the existing job and queues nothing.

A kind with no executor is refused at submission rather than accepted and failed
later. A full queue refuses too, and the record it would have created is failed
rather than left queued forever.

## Poll

`GET /v1/jobs/{id}` reports state, outcome, and attempt count.

A job belonging to another principal answers `404`, the same as an id that does
not exist. Ownership is not distinguishable from absence, so an id cannot be
probed for.

## Notify

Only Discord has a durable place to answer in after the asking turn has gone,
so it is the one transport that gets notified. The notice uses the harness
format and carries the job id, so a member can ask about that run.

The notice is sent on a context detached from the worker, so a shutdown cannot
swallow the one message telling the requester how their job went.

Mention safety applies with empty allowed mentions. It matters more here than on
a reply, because a job speaks without being asked again.

## Cancel

`POST /v1/jobs/{id}/cancel`.

Queued work goes straight to `cancelled`, because nothing started and no
cooperative stop is needed. Running work moves to `cancelling`, the execution
context is interrupted, and the job settles at `cancelled`.

Interruption is immediate rather than polled, so a cancel does not wait out the
job's own timeout. A watcher also polls the record, which covers a cancel
arriving from another process against a shared store.

A job that finishes while a cancellation is in flight records what actually
happened. Releasing an executor after cancellation does not resurrect the job.

## Bounds

One execution is bounded by `Timeout`, thirty minutes by default. Overrunning
it is a failure with the timeout phrase, not a job that runs forever.

The queue depth is bounded. Worker count is fixed, so concurrency is a
deployment decision rather than a function of arrival rate.

Turn-scoped model and tool budgets still apply inside an execution step. Async
is not a way to escape a bound.

An executor that panics fails its own job and leaves the worker running.

## Restart

Only `running` and `cancelling` can be stranded. On start the runtime settles
them with an outcome saying a restart interrupted them, so no record sits live
forever and none later reports success.

## Progress

Progress is advisory. Every update is logged; only an admitted one reaches the
origin, rate limited per job so a chatty job cannot flood a channel. An executor
must never depend on an update being delivered.

See [job telemetry](sirens-echo-jobs-telemetry.md) and
[notices](sirens-echo-notices.md).
