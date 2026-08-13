# Jobs are single-process

The store, the queue, and the worker pool all live inside one process. Nothing
about the job system crosses a process boundary, and that is an assumption
rather than a guarantee anyone enforces.

It is written down because it was not. A second replica was proposed on the
strength of the deployment provisioning a database per lane, and nothing in the
manifests or the code said the harness does not use one for jobs.

## What is actually there

Two stores. `MemoryJobStore` keeps records in a map. `FileJobStore` writes one
JSON record per job to a mounted directory, through a temporary file and a
rename so a crash leaves the previous record rather than a truncated one.

There is no SQL. No `database/sql`, no driver, no `FOR UPDATE SKIP LOCKED`, no
advisory lock, no atomic `UPDATE … RETURNING`. The question of which claiming
strategy the store uses has a fourth answer: none, because one process was
assumed.

## The claim is a process-local mutex

A worker takes a job by transitioning it to running, and the file store guards
that transition with a `sync.Mutex`. That excludes another goroutine in the
same process and nothing else.

## What a second replica would actually break

Not double execution. **The queue is an in-process channel**, so two replicas
hold two separate queues and neither can hand the other's job to a worker.

The failures are quieter than that. A job is visible only to the process that
accepted its submission, so a status read or a cancel routed to the other
replica finds nothing. And if both mounted the same directory, two
process-local mutexes guard nothing between them: concurrent transitions on one
id interleave, and the rename makes it last-writer-wins.

## What would have to change first

A shared queue, a claim that is atomic across processes, and a store both
replicas can read. Each is a real piece of work, and none of them is implied by
provisioning a database.

## See also

- [jobs](sirens-echo-jobs.md) - the state machine and the record.
