# Admission control

Admission decides whether a summon becomes a turn. Every accepted turn costs one Agent Proxy completion
and up to six MCP tool rounds, so **admission is the only place that bounds what a deployment can
spend**. The policy applies at the shared turn boundary, so Discord and the private HTTP path are
governed by one configuration rather than two.

A deployment can join a guild whose members the operator does not moderate. Without admission control
any member could summon the service in a loop and every summon would reach the model, the runtime
having previously queued each summon behind one execution slot with no bound.

## Tiers

Each tier is a token bucket. Burst is how many summons may arrive at once, and the interval is how long
one further token takes to return. Each takes `<burst>/<interval>` or the literal `off`, disabling one
tier leaves the others in force, and an unset variable keeps its packaged default.

* `SIRENS_ECHO_RATE_USER` - `3/30s` - one Discord account or identified HTTP caller.
* `SIRENS_ECHO_RATE_CONTEXT` - `10/10s` - one guild or direct-message channel, so one busy guild cannot
  consume every other guild's budget.
* `SIRENS_ECHO_RATE_GLOBAL` - `20/5s` - the whole process.
* `SIRENS_ECHO_MAX_PENDING` - `8` - summons waiting for the execution slot. Beyond it the runtime sheds
  rather than queues.
* `SIRENS_ECHO_RATE_NOTIFY_EVERY` - `5m` - how often one key is told it was limited.

**A summon is checked against every tier before any tier is charged**, so a request refused by the
global bucket does not silently spend the member's own budget.

Discord gets one short cooldown notice per key per notify window, not one per denied summon, because **a
reply per denial would burn the bot's own Discord message budget and hand a flooder an amplifier**.
HTTP gets `429` with a `Retry-After` header. Both paths record `sirens_echo.admissions` with closed-set
`outcome` and `transport` labels, the outcomes being `accepted`, `denied_user`, `denied_context`,
`denied_global`, and `denied_queue`. **No member-supplied value reaches a label, so a flood cannot
expand metric cardinality.**

Gate evaluation decides from the Gateway payload where it can. Two gates can still need a Discord API
call, an unseen thread whose parent is unknown and a reply whose referenced message was not delivered,
and both are bounded per context so an unscoped channel cannot force a call per message, with scope
decisions cached in both directions.

`SIRENS_ECHO_QUEUE_TIMEOUT` (`30s`) is how long a summon may wait for the execution slot and
`SIRENS_ECHO_REQUEST_TIMEOUT` (`3m`) how long the turn itself may take. **The request budget starts
after the turn acquires the slot**, because a turn that started its budget on arrival would reach the
model with only the remainder. The typing indicator starts when the turn starts running and refreshes
until the reply is sent. Tool results and the completion budget are bounded separately
([the completion budget](sirens-echo-model-call.md)).

The defaults suit a guild the operator does not moderate. Raise them for a trusted guild, and prefer
lowering the context tier over the user tier when the concern is aggregate spend. **Admission bounds
cost, not moderation.**

## Which bucket ages out

The bucket table is capacity-bounded so key churn cannot grow it without limit. **Which key that bound
chooses to drop matters more than the bound.**

**The global bucket sits outside the tracked order entirely**, one fixed key rather than a member of the
rotating population the bound exists to contain. Under insertion-order eviction it was both the earliest
key and the one every admission touches, so **the single bucket that must never go was the one
structurally most likely to go**. Recreating a missing key restores it at full burst, making eviction a
reset rather than a degradation, so a caller who can mint keys could otherwise spend past the configured
global budget by rotating them.

The remaining keys evict **least-recently-used**, which also stops one caller evicting another caller's
partly-spent bucket, since **a bucket that comes back full is a budget that was never spent**. Discord
is unaffected either way, its user keys being author snowflakes a member cannot rotate, so the reachable
case is the tailnet-only HTTP path where the key is `"http:"` plus a caller-asserted header.
`exchangeLimiter`, which bounds agent-to-agent runs per channel, evicts by last-use timestamp already.

## Coalescing
Admission bounds spend by refusing. Coalescing bounds it by **answering several comments in one turn**,
the only lever that lowers the turn rate without turning anyone away.
`SIRENS_ECHO_COALESCE_ENABLED` puts the summon path on it and **defaults off**, so the slot stays the
shipped behaviour and the flag is the rollback. `SIRENS_ECHO_COALESCE_*` in
[the reference](sirens-echo-tuning.md) tunes it.

**Ingress acknowledges every comment before any batching decision exists**, so folding work never folds
the ack, and the buffer **sheds its oldest rather than blocking the gateway**, retracting that mark. The
window opens on the first pending ask, not the newest, so no stream postpones it forever.

**The shard is the member, not the channel**, so two members talking in one place are answered at once
and one member's three rapid comments once, by workers with **one writer per member**. **The comment
that arrives last carries the reply**, the earlier ones folding into it in arrival order and in the
member's own words, read from before the oldest so no comment is history for itself. The turn context
says how many comments the ask carries and that answering it answers all of them, a count and never
an identifier.

Two bounds move while it is on. With no slot to wait for, **`SIRENS_ECHO_COALESCE_CAPACITY` is the
backlog bound** `SIRENS_ECHO_MAX_PENDING` was, the rate tiers unchanged. The ladder does not fire:
**the turn owns its retry and its failure notice**, and only a shutdown dead-letters.
