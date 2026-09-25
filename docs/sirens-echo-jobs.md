# Jobs

A turn is request-scoped. **A job outlives the turn that created it**, so progress, cancellation,
resumption, and per-run history become properties of an object rather than of a conversation.

## The record and its state machine

```
queued ──► running ──► succeeded / failed
   │          ▼
   └──► cancelling ──► cancelled
```

`queued`, `running`, and `cancelling` are live; the rest are terminal. Two edges look odd and are
deliberate: **`queued` goes straight to `cancelled`**, because work that never started needs no
cooperative stop, and **`cancelling` reaches `succeeded` or `failed`**, because a job that finishes
while a cancellation is in flight really did finish. **A move the machine does not list is an error,
never a silent overwrite**, and a move to the same state is a no-op, which keeps a retry idempotent.

The record carries id, idempotency key, requesting principal, kind, origin, state, timestamps, attempt
count, a short outcome phrase, and applied effects. **No prompt, no reply body, no member text**: the
record is deployment-safe by construction, which matters because it outlives the turn and is queryable.
The **principal** is stored from the start even though per-requester authority was not approved in that
batch, because **storing an owner grants nothing and adding one to existing records later is worse**.
The **origin** is how a result reaches the requester after the asking turn is gone, and only Discord has
a durable place to answer in.

**Transports deliver at least once, so the harness owns deduplication.** The idempotency key defaults to
the origin, because Discord redelivers a message under the same id, and the job id is derived from the
key by hash, **so two racing redeliveries produce one id before the store even compares keys**.

`FileJobStore` writes one JSON file per job through a temporary file and a rename, **so a crash
mid-write leaves the previous record rather than a truncated one**. **Only `running` and `cancelling`
can be stranded**, and `RecoverStrandedJobs` moves those to a terminal state with an outcome saying why,
so an interrupted job never sits live forever and never later reports success.

**`queued` is dropped rather than stranded, and the record now says so.** The work queue is a channel
built empty by `Start` and `enqueue` is called only by `Submit`, **so nothing requeues a job a restart
found queued** and an accurate `queued` record is a permanently pending one. `SettleDroppedJobs` moves
those to `failed` under `dropped by a restart`. Requeuing instead was considered and not taken: it is
the larger change, and it needs `Effects` to be load-bearing. See sirens-echo#878.

**Recovery announces every job it settles**, dropped and stranded alike, since a Discord requester never
reads a record. `Attempts` counts executions started, so a resumed
job cannot look like a first run, and `Effects` records what a job already applied, keyed by a step its
kind declares, **so a resumed job skips work it did rather than double-applying it**.

## Lifecycle

`POST /v1/jobs` with a kind and optionally an idempotency key returns **`202` with the id and `queued`
state rather than a finished answer**, so a request that will take minutes does not hold the transport
open, and a redelivery returns the existing job. **A kind with no executor is refused at submission
rather than accepted and failed later**, and a full queue refuses too, with the record it would have
created failed rather than left queued forever. `GET /v1/jobs/{id}` reports state, outcome, and attempt
count, and **a job belonging to another principal answers `404`, the same as an id that does not
exist**, so an id cannot be probed for.

`POST /v1/jobs/{id}/cancel` moves queued work straight to `cancelled` and running work to `cancelling`,
interrupting the execution context. **Interruption is immediate rather than polled**, and a watcher also polls the record for a cancel
from another process.

One execution is bounded by `Timeout`, thirty minutes by default. The queue depth is bounded and worker
count is fixed, **so concurrency is a deployment decision rather than a function of arrival rate**.
Turn-scoped model and tool budgets still apply inside an execution step: **async is not a way to escape
a bound.** An executor that panics fails its own job and leaves the worker running. **Progress is
advisory**: every update is logged, only an admitted one reaches the origin, rate limited per job, and
an executor must never depend on an update being delivered.

## The store and its kinds

`JobStore` is an interface because the deployment picks the backend. `MemoryJobStore` satisfies every
behaviour except durability; `FileJobStore` adds durability on any mounted volume
(`SIRENS_ECHO_JOB_STORE`); `PostgresJobStore` puts the record in a database
(`SIRENS_ECHO_JOB_STORE_DSN`). **The unset default is the quiet one**: a deployment that never sets
either variable is running the memory store, and its jobs do not survive a restart, **correct for a test
and a data-loss boundary anywhere else**.

**The two durable stores are not interchangeable, and the difference is the one a roll exposes.**
`FileJobStore` survives a process restart on the volume it was scheduled onto; `PostgresJobStore`
survives the pod. Under `strategy: Recreate` on a single replica, a roll destroys the pod, **so the file
store's durability depends on the volume outliving it and the database's does not**. **Both variables
set is refused at boot** rather than resolved by precedence, and **connection failure at boot is fatal**
rather than a fallback to memory, which would turn a loud outage into silent data loss.

**`JobKinds` is a closed set.** A kind is a capability, so widening it is a reviewed act rather than
something a caller picks.

## Jobs are single-process

The store, the queue, and the worker pool all live inside one process, **an assumption nothing
enforces**. A worker takes a job by transitioning it to running, and the file store guards that with a
`sync.Mutex`, **which excludes another goroutine in the same process and nothing else**.

**What a second replica would break is not double execution.** The queue is an in-process channel, so
a job is visible only to the process that accepted it, and a status read or cancel routed to the other
replica finds nothing. On a shared directory **two process-local mutexes guard nothing between them**.
A shared queue and a cross-process claim would each have to land first.

## Discord events cross processes

`discord_events` is the one table several processes write, the hand-off from intake to worker
(teable:coilyco/sirens-echo#8269). **`sirens-echo-intake` holds a gateway session and inserts every
event under a key every session derives alike**, `discord:msg:<id>`, `discord:edit:<id>:<edited
timestamp>`, or `discord:interaction:<id>`, with `ON CONFLICT DO NOTHING`, so two intakes and a
connected worker yield one row. It drops only its own posts and link-preview edits, and admission stays
with the worker.

**The worker claims oldest first with `FOR UPDATE SKIP LOCKED`, marking the row in the same statement**,
so a claim is at most once and a second worker takes others. A row stays until the day-old sweep, so
a late duplicate collides with it rather than being answered twice. An interaction past Discord's
three-second deadline, and a message older than `SIRENS_ECHO_DISCORD_EVENT_MAX_AGE`, are dropped and
counted `expired` on `sirens_echo.discord.events`.

`SIRENS_ECHO_DISCORD_QUEUE=true` makes the worker's gateway offer rather than admit, and consume.
`SIRENS_ECHO_DISCORD_GATEWAY=false` then closes that session. The worker learns its identity from
`GET /users/@me`, registers commands from there, and fills its empty state cache with one REST read per
guild and channel. The intake's `/readyz` passes only after READY or RESUMED and a database ping, and never
registers commands. Cutover: queue on, then intake to two, then gateway off.
