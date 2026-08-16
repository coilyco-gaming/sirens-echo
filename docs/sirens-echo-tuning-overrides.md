# Tuning a deployment

Every number this service ships reads an environment variable. There is no
second tier of constants a deployment cannot reach.

[The reference](sirens-echo-knobs.md) is the list, generated from the table
itself by `just knobs`, so it cannot fall behind what the code offers.
`just knobs-check` fails when it has.

## One helper, one behaviour

Each number is declared on one line that binds where the package reads it, the
name that sets it, and what it holds without one:

```go
overridable(&defaultQueueTimeout, "SIRENS_ECHO_QUEUE_TIMEOUT", 30*time.Second)
```

A duration takes Go's spelling, `90s` or `3m` or `1h`. A count takes a plain
integer. Nothing else is accepted, and adding a number without going through
this helper is what `TestEveryTuningNumberLivesInConfigGo` fails on.

## A bad value keeps the default, and says so

Unparsable, zero, or negative applies nothing, for every name alike. A typo
leaves the service on its default rather than on a number nobody chose, which
is the direction that fails safe when a values file is edited under pressure.

Silence there would read as a working override, so the rejected names are
reported on the `capabilities` log line at startup beside the applied ones.

**This used to differ by name.** `REQUEST_TIMEOUT`, `QUEUE_TIMEOUT`, and
`SHUTDOWN_GRACE` were parsed a second time to fill a `Config` field, and that
second reader refused a bad value and failed the load. One name, two readers,
two answers. `Config` now takes what the knob pass produced.

## A derived value is set through its input

Three numbers are expressions of another and have no name of their own:

```
turnProgressEvery     twice turnProgressAfter
turnLongReplyAfter    the wait plus two beats
replyAttachmentBytes  the scratchpad's per-file limit
```

They are recomputed **after** the overrides land. Read before, an override
would move the beat and leave the long-reply threshold on the old number: the
override would appear to work while the threshold deciding whether a reply gets
a thread silently disagreed with it.

## What is not a knob

An algorithm's floor is not a number a deployment sizes. `minNormalizedIDDigits`
and `minEncodedGuardBytes` decide what counts as an identifier rather than how
much of one to allow, so they stay where they are used and no name reaches
them. `TestNoAlgorithmFloorIsOverridable` holds that.

Lowering one from a values file would be a way to switch a control off **while
looking like tuning**, and a loosened guard is indistinguishable from a
configured one from the outside. If one ever needs to move, it moves in a
commit that can be reviewed as what it is.

## Some of these are Discord's numbers, not ours

`SIRENS_ECHO_THREAD_PREFILL_PAGE`, the command-shape bounds, and the reply
limit describe what Discord accepts. Raising one past that fails at Discord
rather than here, and the failure arrives as a rejected send rather than as a
startup error. They are settable because everything is, not because moving them
is a good idea.

## See also

- [tuning](sirens-echo-tuning.md) - every number and why it is that number.
- [the reference](sirens-echo-knobs.md) - the generated list of names.
