# Tuning a deployment

Eight timeouts and one cadence read an environment variable. Everything else in
[tuning](sirens-echo-tuning.md) is a constant on purpose.

```
SIRENS_ECHO_REQUEST_TIMEOUT   the whole turn
SIRENS_ECHO_QUEUE_TIMEOUT     the wait for an execution slot
SIRENS_ECHO_SHUTDOWN_GRACE    how long turns in flight get on restart
SIRENS_ECHO_PROGRESS_AFTER    how long before a turn narrates
SIRENS_ECHO_ROSTER_REFRESH    MCP staleness bound
SIRENS_ECHO_MCP_CONNECT       one MCP handshake
SIRENS_ECHO_MCP_LIST          one tools listing
SIRENS_ECHO_TOOL_CALL         one tool call
```

Each takes a Go duration: `90s`, `3m`, `1h`.

Every one of them is declared with its name and its default on the same line,
where the value lives:

```go
defaultQueueTimeout = overridable("SIRENS_ECHO_QUEUE_TIMEOUT", 30*time.Second)
```

So the answer to "what does this fall back to" is on the line that names it,
and two tests pin the declaration, the write-through table, and this page
against each other.

## A bad value keeps the default, except on three

Unparsable, zero, or negative applies nothing. A typo therefore leaves the
service on its default rather than on a number nobody chose, which is the
direction that fails safe when a values file is edited under time pressure.

`SIRENS_ECHO_REQUEST_TIMEOUT`, `SIRENS_ECHO_QUEUE_TIMEOUT`, and
`SIRENS_ECHO_SHUTDOWN_GRACE` are the exception, because `LoadConfig` reads them
a second time to fill a `Config` field and refuses a value it cannot parse. The
table applies them silently and the load then fails, so a typo on those three
stops the service rather than being ignored. That is the louder direction and
worth knowing, since the two halves disagree about it.

## The derived pair is recomputed

`turnProgressEvery` is twice `turnProgressAfter`, and `turnLongReplyAfter` is
built from both. They are recomputed **after** the overrides are applied.

Read before, an override would move the beat and leave the long-reply
threshold on the old number. That is the failure worth naming: the override
would appear to work, the narration cadence would change, and the threshold
deciding whether a reply gets a thread would silently disagree with it.

## What is deliberately not overridable

```
opaqueSecretRunes      how much of a secret is enough to refuse a reply
minEncodedGuardBytes   the encoded-exfil guard's floor
maxProxyToolNameBytes  a protocol limit, not a preference
```

The first two are security floors. An environment override on them is a way to
switch a control off from a values file **while looking like tuning**, and a
loosened guard is indistinguishable from a configured one from the outside.

If one of these ever needs to move, it moves in a commit that can be reviewed
as what it is.

## The rest

The other constants are algorithm shape rather than deployment tuning —
`completionBudgetStep`, `maxResponseRepairs`, the retry ladder. They are not
overridable because changing them changes behaviour rather than sizing it.

## See also

- [tuning](sirens-echo-tuning.md) - every number and why it is that number.
