# Keeping one requester's scratchpad out of another's

The partition is a hash of the requester rather than the requester with its
punctuation deleted. Stripping is lossy, so any stripping rule shares a
partition between some pair of requesters, and the HTTP requester is a
caller-asserted header, which makes that reachable on purpose. It matters more
since a trimmed tool result spills here automatically, so a colliding caller
would read output the other party never chose to persist.

Hashing also keeps the identifier from being recoverable at all, which is what
a flat predictable name was reaching for. Only an absent requester falls back to
a shared name.

## What a collision would have exposed

Before the tool-result spill a partition held only what a caller deliberately
wrote. Since it, an oversized tool result lands there automatically, so a
colliding caller reads output the other party never chose to persist. The spill
widened the blast radius of the naming defect rather than creating it.

## The cost, stated

An operator can no longer read a requester off a directory listing. That is a
privacy improvement rather than a loss, since attribution lives in the job
record rather than in the filesystem. Every partition name changes on the next
roll, which orphans existing directories. The scratchpad is documented as living
for one rollout, so that is within its contract.

## Provenance inside a partition

Keeping partitions apart says whose files these are. It does not say who wrote
them. A trimmed tool result is saved under `tool-output`, and the model is
refused any write whose first segment is that directory, checked before the path
is cleaned so a spelling cannot smuggle one in.

So a file there was written by the runtime rather than by a model imitating one.
The model keeps the rest of the partition, and a saved result stays readable,
since being readable is the whole reason for saving it.

The runtime reaches that directory through a session method the tool schema
never exposes, rather than through the write tool. Confinement, the per-file
limit, and the per-requester quota all still apply, because it is the same write
path with one check skipped.

## See also

* [Scratchpad](sirens-echo-scratchpad.md) - the filesystem this partitions.
* [Large results](sirens-echo-tool-results.md) - what spills into it.
* [Attribution](sirens-echo-attribution.md) - where the requester is recorded.
