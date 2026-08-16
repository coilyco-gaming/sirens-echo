# The session workspace

A scratchpad partition is a **session**, not a requester. Two members working
in one Discord thread share a workspace, which is what makes it useful in a
community space. See sirens-echo#156 for the decisions this implements.

## What a session is

* **Inside a thread**, the thread is the session. Everyone in it shares one
  workspace.
* **Outside one**, the session is the channel-and-user pairing. There is no
  bounded conversation to share, so it is private.

The identity is hashed before it reaches a path, so no member identifier lands
where the model reads it. A `t-` or `c-` prefix carries the retention rule, so
the collector reads it off the entry rather than out of separate state.

## Why it nests

```
<session>/<requester>/notes.txt
```

A flat rename from requester to session is wrong. The 4 MiB quota is measured
over the partition directory, so renaming it **moves** that bound rather than
adding one, and a member with five live threads would hold five partitions
against a 128Mi `emptyDir` whose overflow evicts the pod.

Nesting gives two things to measure, so both bounds hold:

* **Per requester**, summed across every session they take part in. This is the
  ceiling the volume was sized against, and it survives.
* **Per session**, summed over the whole session directory. This is the new
  bound, and it is what stops an active thread accumulating under a rule that
  never expires it.

## Reading and writing

A write lands in the caller's own subtree. A read tries that subtree first and
then the rest of the session, so `notes.txt` is yours and another member's is
reached by the path a listing shows. Listing the root lists the session.

## Retention

| Session | Collected after |
| --- | --- |
| Thread | 7 days quiet |
| Channel and user | 1 hour idle |

A session's age is the newest **file** in it. Directories are skipped: removing
a file moves its directory's timestamp, so counting them would let eviction keep
a dead session alive. An empty session falls back to its own directory time.

The sweeper runs on a timer and once at startup, and logs each collection with
the session, its kind, and the reason. A directory this scheme did not write is
left alone.

## The cap evicts rather than refusing

Passing the session cap removes the oldest files in **that session** until the
write fits, and names them in the tool result. Refusing would leave a busy
thread read-only. Eviction never reaches another session.

The per-requester ceiling still refuses, because no single file is the right
one to evict when one member is using too much overall.

## The numbers

`SIRENS_ECHO_SESSION_BYTES` is 1 MiB, derived: it has to sit under the 4 MiB
per-requester ceiling or it never binds first, and 4 divided by 1 gives a member
four concurrent full sessions. Nesting keeps the 32-requester arithmetic true.

Every value here is a knob, listed in [the knob reference](sirens-echo-knobs.md).

## See also

* [The scratchpad](sirens-echo-scratchpad.md) - the tools and their bounds.
* [Attachments](sirens-echo-attachments.md) - what an upload lands as.
