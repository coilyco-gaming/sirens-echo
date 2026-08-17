# Mirroring the tool-call trajectory

Deep emits a durable record of what it called into Temporal Cloud. **Metadata
only, and Temporal observes the turn rather than running it.**

Mirroring rather than orchestration, decided with Kai on 2026-08-16. An activity
per tool call would put a round trip in front of a Discord member, and a Temporal
retry policy stacked on the agent-proxy fallback would compound a 502 into a slow
expensive one. Neither happens to something outside the control path.

## The payload is a struct, not a filter

`ToolCallRecord` has five named fields: server, tool, outcome, elapsed millis,
trace id. That is the whole thing.

**It is deliberately not the span's attribute slice.** `StartSpan` is a variadic
passthrough, so copying from it would mirror whatever any caller passed, sight
unseen, forever. Someone adds `attribute.String("message.text", ...)` to a span
one day and member content starts flowing to a third-party SaaS with no change
here and nothing to notice it.

Widening what leaves means adding a field to that struct, a disclosure decision
someone makes on purpose. A test enumerates the fields and fails when the set
changes. The trace id is the one field outside the metric's own triple: it names
no person, and a member is already handed one in a failure notice.

## Tool calls, not spans

Only `RecordToolCall` mirrors. `StartSpan` runs many times per turn and does not.

Agent Proxy logged roughly 68,900 `http receive` spans against about 4,300 real
requests over 30 days, so hooking span-start would point a firehose at a service
that bills per action. One `SignalWithStartWorkflow` per tool call is one action
whether the turn's workflow exists yet or not.

## Never on the turn's path

The send is a non-blocking channel write. A full queue drops.

A single worker owns the only call into the mirror, with a hard timeout, on a
context detached from the turn so a finished turn does not cancel its own audit
record. Errors are swallowed. A panicking client is recovered.

Every one of those paths increments `sirens_echo.mirror.drops`, so an outage is
a number rather than a silence. That is the point: a mirror that fails quietly
is the shape of sirens-echo#137 and sirens-echo#190.

## One workflow per turn

`SignalWithStartWorkflow` keyed on `sirens-deep-trajectory-<trace id>`. The first
tool call starts the workflow and the rest signal it, so a turn's calls arrive as
one ordered trajectory instead of one workflow each.

`ToolTrajectoryWorkflow` accumulates the signals and returns when they stop. It
performs no activity and reaches nothing. Its whole job is to give the records
somewhere durable to land, and it runs on the deployment's worker rather than in
this process.

## Configuration

Off unless a deployment supplies all three of `SIRENS_ECHO_TEMPORAL_HOST`,
`SIRENS_ECHO_TEMPORAL_NAMESPACE`, and `SIRENS_ECHO_TEMPORAL_TASK_QUEUE`.

**A half-filled connection fails at boot.** A typo that quietly turned the mirror
off would be the same silent failure the drop counter exists to prevent. A dial
failure is different and is only logged: Temporal being unreachable must never
stop this service answering.

`SIRENS_ECHO_TEMPORAL_API_KEY` comes from the pod environment. Provisioning is
sirens-echo#444.

Deep only, and the isolated owl-glass surface first. That is a deployment
choice, made by which lane sets the variables, tracked in `coilyco-bridge/deploy`.

## See also

- [telemetry](sirens-echo-observability.md) - the metadata vocabulary this reuses.
- [tuning a deployment](sirens-echo-tuning-overrides.md) - the queue and timeout knobs.
