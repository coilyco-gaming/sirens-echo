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

Only `running` and `cancelling` can be stranded by a crash. `queued` is still
accurate and recovery leaves it alone. `RecoverStrandedJobs` moves the stranded
ones to a terminal state with an outcome saying why, so an interrupted job never
sits live forever and never later reports success.

`Attempts` counts executions started, so a resumed job cannot look like a first
run. `Effects` records what a job already applied, keyed by a step its kind
declares, so a resumed job skips work it did rather than double-applying it.

## The store is an interface

`JobStore` is an interface because the deployment picks the backend.
`MemoryJobStore` satisfies every behaviour except durability, which makes it
right for a test and wrong for a deployment. `FileJobStore` adds durability on
any mounted volume, selected with `SIRENS_ECHO_JOB_STORE`. Leaving that unset
keeps jobs in memory and loses them on restart.

## Kinds

`JobKinds` is a closed set. A kind is a capability, so widening it is a reviewed
act here rather than something a caller picks.

See [configuration](sirens-echo-config.md) and [the service](sirens-echo.md).
