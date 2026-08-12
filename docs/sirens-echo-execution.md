# Executing jobs

A job can check out a repository and run one allowlisted verb against it, in a
workspace no other job can see. This is the unit with the largest blast radius,
so what it relaxes is stated rather than discovered.

## Off by default

No workspace root means no execution at all, which is the shipped posture.
`SIRENS_ECHO_JOB_WORKSPACE` enables it, and the deployment opts in deliberately.

## The admission gate, enforced rather than recommended

Items 5 through 8, per-requester authority and attribution, were not approved.
The consequence is plain: **every job runs under pod authority with no
per-requester attribution**. Whoever asked, the job acts as the same identity
with the same grants, and nothing in the audit trail distinguishes them.

That is acceptable while the admission policy is a direct-message allowlist of
one account, because the requester set and the trusted principal are the same
person. It stops being acceptable the moment the surface widens.

That sequencing note is a gate rather than a comment. `CheckExecutionAdmission`
refuses to build the executing kind when the surface is wider than one account:
any guild entry, more than one direct-message account, the environment path's
open-DM widening, or no policy at all. Opening the guild therefore **disables
execution** rather than silently outrunning its own assumptions.

To have both, items 5 and 6 come first. That is the ordering #145 asked for, now
expressed where it cannot be forgotten.

## What is bounded

* **Wall clock.** One command is bounded inside the job's own budget, and the
  job is bounded by the runner. Two ceilings, the inner one shorter.
* **Output.** Capped before it is read into memory, and truncation is recorded.
  Command output never reaches the outcome phrase, so no build log becomes a
  reply and no token in a log becomes a message.
* **Verbs.** A closed set. A verb is a capability, so widening it is a reviewed
  act in this repository.
* **Repositories.** A closed set mapping a name to a clone URL. An arbitrary URL
  would make the workspace a fetch-anything surface.
* **Environment.** A command gets an explicit environment rather than this
  process's own, so it inherits no token the runtime holds.

## The workspace

One directory per job, named by job id, created on start and removed on every
exit path including failure, cancellation, and panic. A leftover from a previous
attempt is cleared rather than reused, so an attempt starts from a known state.

Two jobs never share one, because the path is derived from an id that is unique
by construction. Removal is idempotent, because a terminal state can be reached
by more than one path.

## What this cannot bound from here

**Network reach.** A process cannot bound its own egress. What a job may reach
is a deployment concern, enforced by NetworkPolicy and the egress proxy, and
this repository can only decline to hand a command any credential. Saying
otherwise here would be claiming a boundary that is not held.

**The pod posture.** Execution needs a writable workspace volume, which relaxes
`readOnlyRootFilesystem` for that path alone. Capabilities stay dropped and no
exec surface is added to the pod. The precise relaxation belongs in the deploy
repository beside the volume that grants it.

## Governed execution

Execution goes through `ward`, the existing governed execution layer. There is
deliberately no second path. `CommandRunner` is an interface so the executor is
testable without a real ward on the machine, and `WardCommandRunner` is the only
implementation that runs anything.

See [the lifecycle](sirens-echo-jobs-lifecycle.md) and
[access](sirens-echo-access.md).
