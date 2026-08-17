# The scratchpad

A requester can keep text files across one conversation, in a partition no other requester can see.
**Working notes, not a filesystem the agent lives on**, and **not** the job workspace in
[executing jobs](sirens-echo-execution.md), which is a checkout an allowlisted verb runs inside. This
one holds text and runs nothing.

No mounted root means no scratchpad tools at all; `SIRENS_ECHO_SCRATCH` enables it. **Absence offers no
tools rather than tools that refuse**, so a deployment that never asked for the capability pays nothing
for it in the prompt, **which matters because every tool schema is resident on every turn**. The backing
volume is an `emptyDir`, so it lives as long as the pod and a rollout erases it: useful across a
conversation without becoming durable state. **It assumes one replica** - a second pod carries a second
volume with nothing binding a requester to either, **so the rule becomes a coin flip** (#489).

**Every requester gets their own directory**, and one requester cannot list, read, or search another's
files. This is what lets the capability exist while the execution guard refuses execution on the same
admission surface: that guard refuses because execution **carries no per-requester attribution**, and
**the scratchpad supplies the attribution instead of asking admission to narrow**. A turn carrying no
principal is refused outright. Attribution reaches the tools through the turn context, **the only
per-turn seam a process-wide tool provider has**, so no tool interface changed.

* **No execution.** Files are written `0600`: the execute bit is denied by the mode rather than
  discouraged by a description.
* **No binaries.** Content must be valid UTF-8, on write and on read.
* **No escape.** A parent segment is refused rather than normalized away, and the target is checked
  after symlinks are followed, because **anchoring a clean at the root would absorb the attempt and
  write somewhere else while reporting the path that was asked for**.
* **No unbounded write.** 256 KB per file and 4 MB per requester, with listing, search results, and
  nesting all capped.

The blast-radius line (#179) is drawn at credentials, infrastructure mutation, code execution or
publication, irreversible external publication, and third-party personal data. **The scratchpad reaches
none of them**: no credential, nothing mutated outside its own volume, nothing executed, nothing
published, and a rollout erases it, its worst case being a requester filling their own 4 MB and being
told so. The line it stays behind is **a writable path into a repository working tree**, which is
deliberately not this and needs its own decision. Four tools: `scratch_list`, `scratch_read`,
`scratch_write`, and `scratch_search`.

## Keeping one requester's out of another's

**The partition is a hash of the requester rather than the requester with its punctuation deleted.**
Stripping is lossy, so any stripping rule shares a partition between some pair of requesters, and the
HTTP requester is a caller-asserted header, **which makes that reachable on purpose**. Hashing also
keeps the identifier from being recoverable at all. Only an absent requester falls back to a shared
name.

**Keeping partitions apart says whose files these are. It does not say who wrote them.** A trimmed tool
result is saved under `tool-output`, and the model is refused any write whose first segment is that
directory, **checked before the path is cleaned so a spelling cannot smuggle one in**. So a file there
was written by the runtime rather than by a model imitating one, while the model keeps the rest of the
partition and a saved result stays readable. The runtime reaches that directory through a session method
the tool schema never exposes, **the same write path with one check skipped**, so confinement, the
per-file limit, and the quota all still apply.

## The session workspace

**A scratchpad partition is a session, not a requester.** Two members working in one Discord thread
share a workspace, which is what makes it useful in a community space (#156). **Inside a thread, the
thread is the session** and everyone in it shares one workspace; **outside one, the session is the
channel-and-user pairing**, because there is no bounded conversation to share. The identity is hashed
before it reaches a path, **so no member identifier lands where the model reads it**, and a `t-` or `c-`
prefix carries the retention rule **so the collector reads it off the entry rather than out of separate
state**.

It nests as `<session>/<requester>/notes.txt`, because **a flat rename from requester to session is
wrong**: the 4 MiB quota is measured over the partition directory, so renaming it **moves** that bound
rather than adding one, and a member with five live threads would hold five partitions against a 128Mi
`emptyDir` **whose overflow evicts the pod**. Nesting gives two things to measure: **per requester**,
summed across every session they take part in, the ceiling the volume was sized against; and **per
session**, summed over the whole session directory, **which is what stops an active thread accumulating
under a rule that never expires it**.

A write lands in the caller's own subtree, and a read tries that subtree first and then the rest of the
session, so `notes.txt` is yours and another member's is reached by the path a listing shows. A thread
session is collected after 7 days quiet and a channel-and-user session after 1 hour idle. **A session's
age is the newest file in it**: directories are skipped, because removing a file moves its directory's
timestamp **and counting them would let eviction keep a dead session alive**. An empty session falls
back to its own directory time. The sweeper runs on a timer and once at startup, logs each collection,
and **leaves alone any directory this scheme did not write**.

**The cap evicts rather than refusing.** Passing the session cap removes the oldest files in **that
session** until the write fits and names them in the tool result, because **refusing would leave a busy
thread read-only**, and eviction never reaches another session. **The per-requester ceiling still
refuses**, because no single file is the right one to evict when one member is using too much overall.
`SIRENS_ECHO_SESSION_BYTES` is 1 MiB, derived: **it has to sit under the 4 MiB per-requester ceiling or
it never binds first**, and 4 divided by 1 gives a member four concurrent full sessions.

## Agent folders

**Everything that pertains to one agent and nothing else lives under `agents/<name>/`. Everything both
agents read stays in `agent/`.** Each agent folder holds `definition.yaml` (its name fixed so the runner
can derive the rest from it), `packs/` (the canonical packs the ward verbs run, with a test failing if a
tracked pack never reaches policy-check), `probes/` (packs run from outside the repository, copied in at
run time **so a committed dataset stays re-derivable** - evidence rather than the canonical set),
`evaluations/` (datasets, each record keeping the paths it was produced with, **so an old one naming a
retired path is history rather than a stale reference**), and `rendered/prompt.txt`.

**The file names repeat across agents on purpose**, so the folder carries the agent rather than the
filename repeating it. `phrases.yaml`, `content-classes.yaml`, `compose/`, `rendered/roles/`, and the
tracker and injection fixtures stay in `agent/`, read by both agents, **so filing them under either one
would be a lie about ownership**. `.agents/skills/` did not move: it is the agentic-os catalog contract
rather than this repository's convention, **so this is the one place the rule is deliberately not
applied**. `docs/` is repo-prefixed rather than agent-prefixed.
