# Admission control

Admission decides whether a summon becomes a turn. Every accepted turn costs one
Agent Proxy completion and up to six MCP tool rounds, so admission is the only
place that bounds what a deployment can spend.

The policy applies at the shared turn boundary, so Discord and the private HTTP
path are governed by one configuration rather than two.

## Why it exists

A deployment can join a guild whose members the operator does not moderate.
Without admission control any member could summon the service in a loop, and
every summon would reach the model. The runtime previously queued each summon
behind one execution slot with no bound.

## Tiers

Each tier is a token bucket. Burst is how many summons may arrive at once, and
the interval is how long one further token takes to return.

* `SIRENS_ECHO_RATE_USER` - `3/30s` - one Discord account or identified HTTP
  caller.
* `SIRENS_ECHO_RATE_CONTEXT` - `10/10s` - one guild or direct-message channel,
  so one busy guild cannot consume every other guild's budget.
* `SIRENS_ECHO_RATE_GLOBAL` - `20/5s` - the whole process.
* `SIRENS_ECHO_MAX_PENDING` - `8` - summons waiting for the execution slot.
  Beyond it the runtime sheds rather than queues.
* `SIRENS_ECHO_RATE_NOTIFY_EVERY` - `5m` - how often one key is told it was
  limited.

Each tier takes `<burst>/<interval>` or the literal `off`. Disabling one tier
leaves the others in force, and an unset variable keeps its packaged default.

A summon is checked against every tier before any tier is charged, so a request
refused by the global bucket does not silently spend the member's own budget.

## What a limited caller sees

Discord gets one short cooldown notice per key per notify window, not one per
denied summon. A reply per denial would burn the bot's own Discord message
budget and hand a flooder an amplifier.

HTTP gets `429` with a `Retry-After` header.

Both paths record `sirens_echo.admissions` with closed-set `outcome` and
`transport` labels. Outcomes are `accepted`, `denied_user`, `denied_context`,
`denied_global`, and `denied_queue`. No member-supplied value reaches a label,
so a flood cannot expand metric cardinality.

## Lookup limiting

Gate evaluation decides from the Gateway payload where it can. Two gates can
still need a Discord API call: an unseen thread whose parent is unknown, and a
reply whose referenced message was not delivered. Those are bounded per
context, so an unscoped channel cannot force a call per message, and scope
decisions are cached in both directions.

## Timeouts

* `SIRENS_ECHO_QUEUE_TIMEOUT` - `30s` - how long a summon may wait for the
  execution slot.
* `SIRENS_ECHO_REQUEST_TIMEOUT` - `3m` - how long the turn itself may take.

The request budget starts after the turn acquires the slot. A turn that started
its budget on arrival would reach the model with only the remainder. The typing
indicator starts when the turn starts running and refreshes until the reply is
sent.

Tool results and the completion budget are bounded separately. See
[the completion budget](sirens-echo-budget.md).

## Tuning

The defaults suit a guild the operator does not moderate. Raise them for a
trusted guild, and prefer lowering the context tier over the user tier when the
concern is aggregate spend. Admission bounds cost, not moderation.
See [the buckets](sirens-echo-admission-buckets.md),
[multiple Discord contexts](sirens-echo-contexts.md),
[the service](sirens-echo.md), and [configuration](sirens-echo-config.md).
