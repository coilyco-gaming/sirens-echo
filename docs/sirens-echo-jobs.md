# The job record

A turn is request-scoped. A job outlives the turn that created it, so progress,
cancellation, resumption, and per-run history become properties of an object
rather than of a conversation.

## The state machine

```
queued ──► running ──► succeeded
   │          │    └──► failed
   │          ▼
   └──► cancelling ──► cancelled
```

`queued`, `running`, `cancelling` are live. The rest are terminal.

Two edges look odd and are deliberate. `queued` goes straight to `cancelled`,
because work that never started needs no cooperative stop. `cancelling` reaches
`succeeded` or `failed`, because a job that finishes while a cancellation is in
flight really did finish, and recording otherwise would be a lie.

A move the machine does not list is an error, never a silent overwrite. A move
to the same state is a no-op, which keeps a retry idempotent and lets an effect
be recorded without pretending the job advanced.

## What the record carries

Id, idempotency key, requesting principal, kind, origin, state, timestamps,
attempt count, a short outcome phrase, and applied effects. No prompt, no reply
body, no member text: the record is deployment-safe by construction, which
matters because it outlives the turn and is queryable.

The **principal** is stored from the start even though per-requester authority
was not approved in this batch. Storing an owner grants nothing, and adding one
to existing records later is worse than carrying it from the beginning.

The **origin** is how a result reaches the requester after the asking turn is
gone. Only Discord has a durable place to answer in, so `Notifiable` is true
there and false for the synchronous transports.

## Identity and idempotency

Transports deliver at least once, so the harness owns deduplication. The
idempotency key defaults to the origin, because Discord redelivers a message
under the same id and "the same message" is the natural unit of "the same
request". A caller may supply its own key instead. The job id is derived from
the key by hash, so two racing redeliveries produce one id before the store even
compares keys, and the store then reports that the job already existed.

## Restart

`FileJobStore` writes one JSON file per job through a temporary file and a
rename, so a crash mid-write leaves the previous record rather than a truncated
one. Opening it loads what is on disk, which makes a restart recoverable rather
than merely non-fatal. Dedup survives with it.

Only `running` and `cancelling` can be stranded by a crash. `RecoverStrandedJobs`
moves those to a terminal state with an outcome saying why, so an interrupted job
never sits live forever and never later reports success.

**`queued` is left alone, and nothing picks it back up.** The work queue is a
channel built empty by `Start`, `enqueue` is called only by `Submit`, and
`JobStore` exposes no query by state, so a job that was accepted and durable but
not yet started is dropped on restart without a notice to its requester. That is
the live gap, tracked separately, and it is the opposite of the double-apply this
section's next paragraph guards against.

`Attempts` counts executions started, so a resumed job cannot look like a first
run. `Effects` records what a job already applied, keyed by a step its kind
declares, so a resumed job skips work it did rather than double-applying it. The
content path writes one per delivered message. Nothing reads a non-empty result
yet, because no path executes a job twice. See sirens-echo#824.

## The store and the kinds

The deployment picks the backend, and the set of kinds is closed. See [the job
store](sirens-echo-jobs-store.md).

See [the lifecycle](sirens-echo-jobs-lifecycle.md).
