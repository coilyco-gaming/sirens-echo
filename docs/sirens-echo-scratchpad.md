# The scratchpad

A requester can keep text files across one conversation, in a partition no other
requester can see. Working notes, not a filesystem the agent lives on.

This is not the job workspace in [executing jobs](sirens-echo-execution.md).
That one is a checkout an allowlisted verb runs inside. This one holds text and
runs nothing, and the two must not be confused.

## Off by default

No mounted root means no scratchpad tools at all. `SIRENS_ECHO_SCRATCH` enables
it and the deployment opts in deliberately.

Absence offers **no tools**, rather than tools that refuse. A deployment that
never asked for the capability pays nothing for it in the prompt, which matters
because every tool schema is resident on every turn.

## One rollout, not one turn and not forever

The backing volume is an `emptyDir`, so the scratchpad lives as long as the pod
and a rollout erases it: no claim to restore, no eviction policy, and useful
across a conversation without becoming durable state. **It assumes one
replica** — a second pod carries a second volume with nothing binding a
requester to either, so the rule becomes a coin flip. See sirens-echo#489.

## Partitioned per requester

Every requester gets their own directory, named from their account id. One
requester cannot list, read, or search another's files.

This is what lets the capability exist while
[the execution guard](sirens-echo-execution.md) refuses execution on the same
admission surface. That guard refuses because execution *carries no
per-requester attribution*, and once a guild is admitted the requester set is no
longer one account. The scratchpad supplies the attribution instead of asking
admission to narrow. A turn carrying no principal is refused outright, because
absence is denial here as everywhere else.

Attribution reaches the tools through the turn context, which is the only
per-turn seam a process-wide tool provider has. No tool interface changed.

## What it will not do

* **No execution.** Files are written `0600`. The execute bit is denied by the
  mode rather than discouraged by a description.
* **No binaries.** Content must be valid UTF-8, on write and on read.
* **No escape.** A parent segment is refused rather than normalized away, and
  the target is checked after symlinks are followed. Anchoring a clean at the
  root would absorb the attempt and write somewhere else while reporting the
  path that was asked for, which hides it rather than answering it.
* **No unbounded write.** 256 KB per file and 4 MB per requester. Listing and
  search results are capped, and nesting is capped.

## Where it sits on the blast-radius line

[Issue 179](https://forgejo.coilysiren.me/coilyco-gaming/sirens-echo/issues/179)
draws the line at credentials, infrastructure mutation, code execution or
publication, irreversible external publication, and third-party personal data.

The scratchpad reaches none of them. It holds no credential, mutates nothing
outside its own volume, executes nothing, publishes nothing, and a rollout
erases it. Its worst case is a requester filling their own 4 MB and being told
so.

The line it stays behind is a **writable path into a repository working tree**,
item 3. This is deliberately not that, and a checkout needs its own decision.

## Tools

* `scratch_list` - list files, optionally under a subdirectory.
* `scratch_read` - read one UTF-8 text file.
* `scratch_write` - create or replace one UTF-8 text file.
* `scratch_search` - find matching lines, case-insensitively.

## See also

* [The session workspace](sirens-echo-session-workspace.md) - a shared thread.
* [Partitions](sirens-echo-scratchpad-partitions.md) - keeping requesters apart.
* [Executing jobs](sirens-echo-execution.md) - the other, unrelated workspace.
