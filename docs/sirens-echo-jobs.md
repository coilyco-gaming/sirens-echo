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

**`queued` is left alone, and nothing picks it back up.** The work queue is a channel built empty by
`Start`, `enqueue` is called only by `Submit`, and `JobStore` exposes no query by state, so **a job that
was accepted and durable but not yet started is dropped on restart without a notice to its requester**.
That is the live gap, tracked separately. `Attempts` counts executions started, so a resumed job cannot
look like a first run, and `Effects` records what a job already applied, keyed by a step its kind
declares, **so a resumed job skips work it did rather than double-applying it**.

## Lifecycle

`POST /v1/jobs` with a kind and optionally an idempotency key returns **`202` with the id and `queued`
state rather than a finished answer**, so a request that will take minutes does not hold the transport
open, and a redelivery returns the existing job. **A kind with no executor is refused at submission
rather than accepted and failed later**, and a full queue refuses too, with the record it would have
created failed rather than left queued forever. `GET /v1/jobs/{id}` reports state, outcome, and attempt
count, and **a job belonging to another principal answers `404`, the same as an id that does not
exist**, so an id cannot be probed for.

Only Discord gets notified, being the one transport with a durable place to answer in. The notice
carries the job id and **is sent on a context detached from the worker, so a shutdown cannot swallow the
one message telling the requester how their job went**. Mention safety applies with empty allowed
mentions, **which matters more here than on a reply, because a job speaks without being asked again**.

`POST /v1/jobs/{id}/cancel` moves queued work straight to `cancelled` and running work to `cancelling`,
interrupting the execution context. **Interruption is immediate rather than polled**, so a cancel does
not wait out the job's own timeout, and a watcher also polls the record, covering a cancel arriving from
another process against a shared store.

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
set is refused at boot** rather than resolved by precedence, because a lane that quietly ran one of two
configured stores would put the jobs somewhere nobody was looking. **Connection failure at boot is fatal
for the same reason**: falling back to memory on an unreachable database would turn a loud outage into
the silent data loss the durable store was chosen to prevent.

The Postgres schema is one table created idempotently on first boot, **which is the whole migration
story**. The record is one `JSONB` column, so the Go struct stays authoritative and adding a field needs
no migration. The four flat columns beside it are a projection existing only because the list queries
would otherwise scan; **nothing reads them back into a `Job`, so they cannot become a second source of
truth**, and they are rewritten on every write rather than only on insert, because `BindJobToThread`
rebinds through `Update` **and a projection tracking inserts alone would answer `ListByThread` from a
stale value**. `Transition` and `Update` read the row `FOR UPDATE` inside a transaction, **the lock
being what keeps a refused move from leaving a partly-applied record**. Its SQL is covered by tests that
need a live database and skip under `just test`, with a CI step running exactly that set against a
scratch service container.

**`JobKinds` is a closed set.** A kind is a capability, so widening it is a reviewed act here rather
than something a caller picks: an open set would let a caller name a kind the service has never been
reviewed for, **and the review is the only thing standing between a job name and the work it
authorises**.

## Jobs are single-process

The store, the queue, and the worker pool all live inside one process, **and that is an assumption
rather than a guarantee anyone enforces**. It is written down because it was not: a second replica was
proposed on the strength of the deployment provisioning a database per lane, **and nothing in the
manifests or the code said the harness does not use one for jobs**. A worker takes a job by
transitioning it to running, and the file store guards that with a `sync.Mutex`, **which excludes
another goroutine in the same process and nothing else**.

**What a second replica would break is not double execution.** The queue is an in-process channel, so
two replicas hold two separate queues and neither can hand the other's job to a worker. **The failures
are quieter**: a job is visible only to the process that accepted its submission, so a status read or a
cancel routed to the other replica finds nothing, and if both mounted the same directory, **two
process-local mutexes guard nothing between them**, so concurrent transitions interleave and the rename
makes it last-writer-wins. A shared queue, a claim atomic across processes, and a store both replicas
can read would each have to land first, **and none is implied by provisioning a database**.
