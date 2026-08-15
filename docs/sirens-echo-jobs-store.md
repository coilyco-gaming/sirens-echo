# The job store and its kinds

Where a job record lives and what kinds of job may exist. What a record holds
is [the job record](sirens-echo-jobs.md).

## The store is an interface

`JobStore` is an interface because the deployment picks the backend.

`MemoryJobStore` satisfies every behaviour except durability, which makes it
right for a test and wrong for a deployment. `FileJobStore` adds durability on
any mounted volume, selected with `SIRENS_ECHO_JOB_STORE`. `PostgresJobStore`
puts the record in a database, selected with `SIRENS_ECHO_JOB_STORE_DSN`.
Neither variable set keeps jobs in memory.

The unset default is worth stating plainly, because it is the quiet one: a
deployment that never sets either variable is running the memory store, and its
jobs do not survive a restart. That is correct for a test and is a data-loss
boundary anywhere else.

All three are [single-process](sirens-echo-jobs-single-process.md). None
coordinates across replicas, so the store choice does not make the job system
horizontally scalable, only durable.

## What the two durable stores survive

They are not interchangeable, and the difference is the one a roll exposes.

`FileJobStore` survives a process restart on the volume it was scheduled onto.
`PostgresJobStore` survives the pod. Under `strategy: Recreate` on a
single-replica deployment, a roll destroys the pod, so the file store's
durability depends on the volume outliving it and the database's does not.

Both variables set is refused at boot rather than resolved by precedence. A
lane that quietly ran one of two configured stores would put the jobs somewhere
nobody was looking, which is the failure this store closes rather than moves.

Connection failure at boot is fatal for the same reason. Falling back to memory
on an unreachable database would turn a loud outage into the silent data loss
the durable store was chosen to prevent.

## The Postgres schema

One table, created idempotently on first boot, which is the whole migration
story. The record is one `JSONB` column, so the Go struct stays authoritative
and adding a field to `Job` needs no migration.

The four flat columns beside it are a projection of that record and exist only
because `Get`, `ListByPrincipal`, and `ListByThread` would otherwise scan.
Nothing reads them back into a `Job`, so they cannot become a second source of
truth, and they are rewritten on every write rather than only on insert:
`BindJobToThread` rebinds `Origin.ThreadID` through `Update`, and a projection
that tracked inserts alone would answer `ListByThread` from a stale value.

`Transition` and `Update` read the row `FOR UPDATE` inside a transaction. The
read and the write are two statements, and the lock is what keeps a refused
move from leaving a partly-applied record.

Its SQL is covered by the tests named in `.ward/test-skips.allow`, which need a
live database and therefore skip under `ward exec test`. The `job store SQL` CI
step runs exactly that set against a scratch service container, which is what
keeps them from being an allowlist entry that never runs anywhere. See
[every skip is a reviewed line](sirens-echo-test-skips.md).

## Kinds

`JobKinds` is a closed set. A kind is a capability, so widening it is a reviewed
act here rather than something a caller picks.

Closed rather than open is the whole control. An open set would let a caller
name a kind the service has never been reviewed for, and the review is the only
thing standing between a job name and the work it authorises.

## See also

- [the job record](sirens-echo-jobs.md) - state machine and identity.
- [the lifecycle](sirens-echo-jobs-lifecycle.md) - how a job moves.
