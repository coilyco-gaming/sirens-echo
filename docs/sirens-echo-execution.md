# Executing jobs

A job can check out a repository and run one allowlisted verb against it, in a workspace no other job
can see. **This is the unit with the largest blast radius, so what it relaxes is stated rather than
discovered.** No workspace root means no execution at all, which is the shipped posture, and
`SIRENS_ECHO_JOB_WORKSPACE` enables it.

Items 5 through 8, per-requester authority and attribution, were not approved, and the consequence is
plain: **every job runs under pod authority with no per-requester attribution.** That was acceptable
while admission was a direct-message allowlist of one account, **because the requester set and the
trusted principal were the same person**. `CheckExecutionAdmission` makes that a gate rather than a
comment: with no grant table it refuses any surface wider than one account, whether a guild entry,
several direct-message accounts, an admitted counterpart agent, the environment path's open-DM
widening, or no policy. **A declared grant table is what that rule stood in for**, so once it exists a
wider surface is bounded by grants, and the guard then asks only that some principal be granted
`ward-exec`, **since a table granting it to nobody would start a runner that can never run**.

* **Wall clock** - one command is bounded inside the job's own budget and the job by the runner: two
  ceilings, the inner one shorter.
* **Output** - capped before it is read into memory, with truncation recorded. **Command output never
  reaches the outcome phrase**, so no build log becomes a reply and no token in a log becomes a message.
* **Verbs** - a closed set, because a verb is a capability and widening it is a reviewed act here.
* **Repositories** - a closed set mapping a name to a clone URL, because **an arbitrary URL would make
  the workspace a fetch-anything surface**.
* **Environment** - a command gets an explicit environment rather than this process's own, **so it
  inherits no token the runtime holds**.

**One directory per job**, named by job id, created on start and removed on every exit path including
failure, cancellation, and panic. A leftover from a previous attempt is cleared rather than reused, so
an attempt starts from a known state, two jobs never share one because the path derives from an id
unique by construction, and removal is idempotent because a terminal state can be reached more than one
way.

**A process cannot bound its own egress.** What a job may reach is a deployment concern enforced by
NetworkPolicy and the egress proxy, and this repository can only decline to hand a command any
credential: **saying otherwise here would be claiming a boundary that is not held**. Execution also
needs a writable workspace volume, which relaxes `readOnlyRootFilesystem` for that path alone, while
capabilities stay dropped and no exec surface is added to the pod. Execution goes through `ward`, the
existing governed layer, and **there is deliberately no second path**: `CommandRunner` is an interface
so the executor is testable without a real ward, and `WardCommandRunner` is the only implementation that
runs anything.

## Work continuing past the turn

`Sirens Echo will keep watching the server` is not a claim about the past, **so none of the grounding
gates read it**, and no tool call can ground it either. A turn ends when the reply is sent and this
runtime holds no scheduler, **so the promise names something no code will ever do**, and the
member-visible outcome is someone waiting for a message that is never coming, **which reads as being
ignored rather than as a failure**.

The pattern is the `no-continuing-work-claim` case in `agents/echo/packs/evaluation.yaml`, **shared with
the runtime rather than copied into it**. `TestContinuingWorkClaimIsPinnedToTheDeploymentGate` loads the
pack and asserts the two are character-identical, so editing either alone fails the suite: **two copies
of one definition drift quietly, and the failure mode is asymmetric**, the gate passing while the
runtime stops matching or the reverse, **and whichever direction nobody is watching is the one that
costs a member an answer**. Before this check the gate held the definition alone, **so the shape failed
a build and shipped at runtime**.

It requires a named subject followed by `is now` or `will`, so `Sirens Echo will not keep watching the
server` does not match and neither does prose about the game world. **A grounding error fails the turn
with no repair loop, so under-firing is the correct direction.** **The verb list is the second bound and
it was too narrow**, built from the promise-to-keep-watching defect, so `Sirens Echo is now searching
the tracker` escaped with the right subject and tense and no matching verb; `search`, `look up`,
`query`, `retrieve`, and `fetch` were added for #341 at zero false positives across twelve correct
replies. **The subjectless form still escapes**, and **requiring the subject is what keeps `The Eco app
is now tracking prices` clean**. English-only, inherited from the gate's pattern (issue 253).

## Duplicate work

**A claim reserves an issue number. The collision surface is files**, and two issue numbers can point at
one function, so claiming correctly does not stop two agents building the same change (#552, #690).
Measured across one seat in ninety minutes, six changes were built and beaten, **every one claimed on
its issue first with no claim jumped**. **The gap is two to five minutes**: in each case the competing
change merged **after** the branch was cut and **before** the push, **so the guard is not a better
claim, it is a fresher read**. Fetch `origin/main` immediately before the first edit, because
**branching is not close enough when the base moves several times an hour**, and look for the thing
before building it.

## Shutdown

**A restart is not a failure, and a member whose turn it interrupts is entitled to hear which one it
was.** SIGTERM reached the HTTP listener and nothing else, every Discord turn being rooted at
`context.Background()`, **so shutdown could not see one, wait for one, or tell one to stop**. They now
descend from a root the service owns, and shutdown goes one way: stop admitting; wait out the grace;
cancel the rest **naming the restart as the cause**; let them send their notice **while the gateway is
still open**; then close the session, the MCP connections, and the job runner. **That close was always
last. What was missing is everything above it.**

Only `context.Cause` separates a restart from a deleted message: `service restarting, retry shortly` is
actionable where `turn timed out` **would blame the member's question for a deploy**. The metric splits
on `shutdown` rather than `stage_failed`, **so a rollout does not read as an outage**. The 15 second
`SIRENS_ECHO_SHUTDOWN_GRACE` is what fits the pod's kill window.

## The turns the drain cannot reach

The grace rescues only quick turns against a p95 of 182 seconds, and sees nothing of a SIGKILL or an OOM
kill. **The loss is structural, so the goal is making it visible** (sirens-echo#989).

So the mark is written at the **start**, a shutdown-time write depending on the least reliable moment in
the process's life and catching only the graceful case. A turn records itself before its first model
call, keyed by the message id and carrying the channel, the author, the start, and **a random identity
for the run that wrote it**, then clears it when the turn ends, success and failure alike. **The next
boot sweeps every record another run left**, which is why the tests name no signal.

A swept record lands on `sirens_echo.turns` as `outcome=interrupted`, **the counter an ordinary turn
already lands on**, and a reply tells the room the summon was dropped. Taking and clearing is one step,
**so a second boot cannot repeat it**, and it never re-answers, a replay risking the double answer
`Recreate` exists to prevent. The log follows the store [jobs](sirens-echo-jobs.md) already selected.
