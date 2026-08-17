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
chooses to drop turns out to matter more than the bound.**

Eviction used insertion order, and `global` is both the earliest-created key and the one every admission
touches, so **the single bucket that must never go was the one structurally most likely to go**, and
churn at the tail evicted the head. Recreating a missing key restores it at full burst, so eviction was
a reset rather than a degradation: a caller who can mint keys could spend past the configured global
budget by rotating them, and on the HTTP path the key is `"http:"` plus a caller-asserted header. The
global bucket is now outside the tracked order entirely, one fixed key rather than a member of the
rotating population the bound exists to contain.

The remaining keys are evicted **least-recently-used**, which is what the bound was described as doing
and was not. That also stops one caller evicting another caller's partly-spent bucket, the quieter
version of the same defect, since **a bucket that comes back full is a budget that was never spent**.

Discord is unaffected either way, its user keys being author snowflakes a member cannot rotate, so the
reachable case was the tailnet-only HTTP path and this bounded what an already-authorized caller could
spend rather than exposing anything. `exchangeLimiter`, which bounds agent-to-agent runs per channel,
evicts by last-use timestamp already and its keys are channel identifiers, so it never shared the defect.

## Errors and the decimated sample

A pass rate is computed over attempts that **returned content**. Errors are excluded from the
denominator, which is correct, because a 502 or an empty completion is a fact about the substrate rather
than a behaviour, and counting one as a behavioural failure would corrupt every rate. **The consequence
is that a clean verdict can rest on far fewer runs than the case declared.**

A case with `runs: 5` reported `passed: 2, attempts: 2, errors: 3`, read as 100 percent. Three attempts
returned empty content after the proxy exhausted a 3600 token budget and escalated twice, so the pass
rate was 100 percent of what was scored and 40 percent of what was asked for, **and nothing in the
headline said so** (sirens-echo#325). Now the breach line names how many declared runs errored and were
excluded, and a `rate.sample.decimated` warning is logged for **any** case with errors, so a case that
passed surfaces it too, on stderr rather than in the dataset stream. Read `errors` beside `attempts` and
`runs`.

**This does not make the rate more reliable.** A behaviour rate over 2 attempts is a weak measurement
whether or not the reader can see that 3 attempts vanished, and the promotion arithmetic in
[the rate pack](sirens-echo-rate.md) applies to the **scored** attempts rather than the declared `runs`.
**No error ceiling gates anything**, because failing a verdict on an error rate needs somebody to decide
what rate is acceptable, which is a live-operations judgement rather than a measurement one, and
inventing a default with no evidence would be the certifying-rather-than-measuring failure in a new
place.
