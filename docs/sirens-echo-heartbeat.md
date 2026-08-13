# A positive signal, so silence means something

Echo emitted nothing between 06:00 and 07:36 one night, and three explanations
fit that evidence equally: nobody messaged it, Discord ingress stopped, or
something upstream of the first log line failed. All three produce identical
telemetry, which is none.

Absence of logs is not evidence. It is indistinguishable from absence of work.

## What the beat carries

One record a minute while the gateway session is open, with counts since the
last beat rather than totals, so it reads as a rate and needs no baseline.

| field | separates |
| --- | --- |
| the record existing | a live process from a stopped one |
| `messages_observed` | ingress arriving from ingress stopped |
| `turns_admitted` | messages that summoned from messages that did not |
| `replies_sent` | turns that answered from turns that failed |

Observed is counted **before** eligibility, because every message being
ineligible and no message arriving are different failures with the same
downstream shape.

## What it still cannot separate

A quiet guild and a stopped gateway both look like a live process observing
nothing. Telling those apart needs a signal from Discord rather than from here,
which is a larger change than this.

What it does close is the case this was filed for: turns arriving and none
being answered, which ran 2.5 hours with nothing detecting it.

## Why this is the shape an alert can use

Absence of a positive signal is evidence in a way absence of logs is not. No
heartbeat in five minutes is a rule that works. The alert itself is deferred
and belongs to whoever owns alerting; this is the signal it keys on, and the
signal has to exist first.

## Bounds

No member identity, no channel, closed-set fields only. The beat runs while the
session is open and stops with it, so it cannot outlive the thing it reports
on, and a deployment with no gateway carries no heartbeat at all.

## See also

* [Delivery failures](sirens-echo-delivery-failures.md) - why a ready reply did
  not land.
* [Exceptions](sirens-echo-exceptions.md) - the fault split an alert reads.
