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

## See also

* [Scratchpad](sirens-echo-scratchpad.md) - the filesystem this partitions.
* [Large results](sirens-echo-tool-results.md) - what spills into it.
* [Attribution](sirens-echo-attribution.md) - where the requester is recorded.
