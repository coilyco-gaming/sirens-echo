# Which rate-limit bucket ages out

The bucket table is capacity-bounded so key churn cannot grow it without limit.
Which key that bound chooses to drop turns out to matter more than the bound.

## The global bucket never ages out

Eviction used insertion order, and `global` is both the earliest-created key and
the one every admission touches. So the single bucket that must never go was the
one structurally most likely to go, and churn at the tail evicted the head.

Recreating a missing key restores it at full burst, so eviction was a reset
rather than a degradation. A caller who can mint keys could therefore spend past
the configured global budget by rotating them, and on the HTTP path the key is
`"http:"` plus a caller-asserted header.

The global bucket is now outside the tracked order entirely. It is one fixed key
rather than a member of the rotating population the bound exists to contain, so
exempting it costs nothing.

## Eviction is by recency, not arrival

The remaining keys are evicted least-recently-used, which is what the bound was
described as doing and was not. That also stops one caller evicting another
caller's partly-spent bucket, which is the quieter version of the same defect: a
bucket that comes back full is a budget that was never spent.

## Scope

Discord is unaffected either way. Its user keys are author snowflakes, which a
member cannot rotate. The reachable case was the private HTTP path, whose
listener is tailnet-only, so this bounded what an already-authorized caller
could spend rather than exposing anything.

## The other eviction in the runtime

`exchangeLimiter` bounds agent-to-agent runs per channel and evicts by the run's
last-use timestamp, so it is already least-recently-used and does not share this
defect. Its keys are channel identifiers, which a member cannot mint either.

## See also

* [Admission control](sirens-echo-admission.md) - the tiers and what each bounds.
* [HTTP entrypoint](sirens-echo-http.md) - why the caller header is not a trust boundary.
