# The job store and its kinds

Where a job record lives and what kinds of job may exist. What a record holds
is [the job record](sirens-echo-jobs.md).

## The store is an interface

`JobStore` is an interface because the deployment picks the backend.

`MemoryJobStore` satisfies every behaviour except durability, which makes it
right for a test and wrong for a deployment. `FileJobStore` adds durability on
any mounted volume, selected with `SIRENS_ECHO_JOB_STORE`. Unset keeps jobs in
memory.

The unset default is worth stating plainly, because it is the quiet one: a
deployment that never sets the variable is running the memory store, and its
jobs do not survive a restart. That is correct for a test and is a data-loss
boundary anywhere else.

Both are [single-process](sirens-echo-jobs-single-process.md). Neither
coordinates across replicas, so the store choice does not make the job system
horizontally scalable, only durable.

## Kinds

`JobKinds` is a closed set. A kind is a capability, so widening it is a reviewed
act here rather than something a caller picks.

Closed rather than open is the whole control. An open set would let a caller
name a kind the service has never been reviewed for, and the review is the only
thing standing between a job name and the work it authorises.

## See also

- [the job record](sirens-echo-jobs.md) - state machine and identity.
- [the lifecycle](sirens-echo-jobs-lifecycle.md) - how a job moves.
