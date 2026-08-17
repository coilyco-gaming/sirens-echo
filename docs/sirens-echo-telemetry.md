# Telemetry beyond the turn

What the harness itself did, what a job did, what an evaluation run costs, and how to count discovery.

## Recording what the harness itself did

A tool call the model made was fully recorded. **A command this service executed was not**, and neither
was fetching a member's upload: a read-only `eco__get_market` left a trace and running a process left
nothing, **absent from the record by being absent from everything**.

Workspace command execution now emits a `job.command` span and `sirens_echo.commands`, counting
`command.verb` against an outcome. **The verb is the label** - this repository's own closed set - **and
arguments are not**, because a clone argument is a repository URL. Three outcomes: `ok` exited zero,
`exited` exited non-zero (**the code goes on the span, not the metric, since 0 to 255 is cardinality a
closed label does not want**), and `did_not_run`. The span carries duration, the truncation flag, and
the job id, **and the output never reaches any of them**.

**The bug the timeout test found.** `exec.CommandContext` kills the direct child on expiry but does not
close the pipe, and `CombinedOutput` waits for every writer to it, **a grandchild included**, so a shell
that forks rather than execs outlived its own deadline: **30s measured against a 50ms timeout**.
`command.WaitDelay = commandKillGrace` bounds the wait after the kill and closes the pipes. **The orphan
is not reaped** - this unblocks the harness rather than killing the process tree (#892).

**None of this reaches the Temporal mirror**, deliberately: the mirror keys off `RecordToolCall` alone,
**so a new `Record*` does not widen what leaves this process**, and **a command verb is closer to
content than `mcp.tool.name` is**. `TestNeitherEffectReachesTheTemporalMirror` holds the line,
**asserting a tool call does mirror in the same run so it cannot pass by delivering nothing** (#890).

## Job-scoped telemetry

Service-scoped telemetry answers "how is the service doing". Once work is durable the question that
matters is **"what happened in that run"**, and that needs a different key. Traces carry `service.name`
and logs carry `k8s.namespace.name`, **neither addressable by unit of work**.

The job id rides the context, so it reaches both without every call site threading it. Spans carry
`sirens_echo.job.id` **not only on the root**: `StartSpan` adds the attribute whenever the context is
inside a job, so `model.chat`, `mcp.tool.call`, and everything else are queryable by id directly rather
than only by walking down from the root. Logs carry `job_id` as a row field. **A span or a row outside a
job carries no job id at all**, because an attribute that is sometimes absent is more useful than one
that is sometimes wrong. Given a job id and nothing else you get every span, every log row, and the
record itself, **with service and namespace needed for none of the three**.

## Evaluation telemetry is its own service

An evaluation run exports to the same receiver as the deployment and reports as `sirens-echo-eval`,
**and that separation is load-bearing rather than tidy**. The eval binary never set `InstanceName`, so
it resolved to the `sirens-echo` default, **and one binary serves both profiles**, so `eval-echo`,
`eval-deep`, `board-deep`, and `rate-deep` all reported as the Echo deployment. `service.name =
sirens-echo` was therefore **the production deployment plus every evaluation run of both profiles**, and
measured over 24 hours that was **898 spans against 169 real turns**. **Every number taken from that
service was wrong in the same direction and none of them looked wrong**: error rate, latency, token
spend, and cache-hit ratio all read as plausible.

**Why nobody caught it**: `sirens-deep` looked clean at almost exactly one lookup per turn, **and it
looked clean because its evaluation traffic was not missing, it was being billed to the other service**.
A contaminated service and a clean one is a much less alarming shape than two contaminated ones, **so
the clean one reassured rather than prompting a question** (#533). An evaluation run still opens no
`community.turn`, so its `mcp.tools.list` spans are roots whose traces contain only themselves. The two
evaluated profiles are not distinguished, **there being no lowercase slug on a definition to derive one
from**.

## What a turn costs

`promptBudgets` ratchets the system prompt, but every request also carries a turn context, **and until
now nothing measured it**. The variable half is the conversation window plus the member's message:
history author at 80 runes and content at 1000 runes, both times `max_context_messages`, plus one
current message at 2000 runes. **At the tracked window of 12 a worst-case turn assembles 15,248 bytes
against an Echo system prompt of roughly 19,900**, so the unmeasured half was about 43% of the request,
and raising `max_context_messages` from 12 to 30 **cost nothing in any tracked budget while being paid
on every turn forever**.

**Why the window is 12**: issue 194 records a fixed window plus fetch-on-demand backfill with the size
left open, and it was already 12. **The fixed window should be the cheapest thing that makes ordinary
continuity work**, because raising it buys context for every turn including the self-contained ones that
need none. A turn that genuinely needs more should reach through backfill, **where the cost falls only
on the turns that ask**. `worstCaseTurn` feeds entries far larger than the caps, **so it measures what
the caps permit rather than what a caller happened to send**, and the window is read from the tracked
definitions. A second check keeps the truncation honest, **because a cap that stopped truncating would
not fail a ceiling on its own**. It does not measure token count - bytes are a proxy, **but checkable
offline with no model and no network** - and it does not measure tool results.

## Tool discovery telemetry

**The span is the lookup, not the round trip.** `mcp.tools.list` wraps the call site, so it is emitted
once per turn whether or not anything reached the network, **and before the roster cache those were the
same number**. Counting spans after the cache landed reported one listing per turn, **the signature of
the defect the cache was built to fix**, while the service was making about five round trips in three
hours: eleven of sixteen spans in that window never left the process.

So the span says which it was: `mcp.tools.configured`, `mcp.tools.reached` (how many went to the network
**including a failed connect**), `mcp.tools.listed`, and `mcp.tools.cached`, true when the roster is
non-empty and none went out. **Count round trips with `reached`. Do not count spans, and do not read the
duration.** Reaching the network and completing a listing are different, **and the difference is the
outage**: a connect that fails is a round trip that listed nothing, and reporting it as cached **would
assert the comfortable answer on the one turn that matters** (#540). `cached` is all-or-nothing over the
roster, the cached count being `configured - reached` (#534), and **an empty roster reports
`cached=false`**, because nothing was served from cache when there is nothing to cache.

Discovery covers a connect and three listings per server, and the whole roster used to sit under one
span, **so a rejection was an `HTTP POST` carrying a URL and nothing else**. `mcp.server.discovery` now
wraps one server's round trips and carries `mcp.server.name`, with `mcp.discovery.stage` moving through
`connect`, `tools`, `resources`, and `prompts`, **so a failure's stage is the span's last value** (#139).

**A tool reporting its own failure now carries error status.** 51 did in twelve hours against one error
span of 158, so every generic error query missed them (#873). The type is
`sirens_echo.mcp.tool_reported_error`, **the class rather than the tool's own words**, since a span
carries no bodies, and a deadline splits out as `tool_call_timed_out`. Neither fails the turn.
