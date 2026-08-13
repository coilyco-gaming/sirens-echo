# A settled anomaly, in order

2026-08-13, recorded because it followed the evidence order before that order
was written down. The order itself is [settling an
anomaly](sirens-echo-anomaly-evidence.md). See [issue
258](https://forgejo.coilysiren.me/coilyco-gaming/sirens-echo/issues/258).

## Lead

Kai reported that Sirens Deep was down. Correct that something was wrong, and
wrong about what: the service was answering.

## The false trail

The service's own account was `model backend unavailable, retry shortly`. Taken
as the explanation, that sends an operator to the inference tier to debug a
backend that was healthy.

This is the expensive shape. The account was not a lie and not a guess. It was
the service reporting what it had been told to report, and the report was
written by someone who assumed that failure class.

## Settlement

Ops reconstructed nine model request and response pairs, every one carrying
`status: 200`, then a turn failure emitted 74 to 93 microseconds after the last
good response.

**A sub-100 microsecond gap is in-process handling.** It cannot be a network
call, a timeout, or an upstream outage. That single number settles the question
without needing to know what the internal cause was, which is why it was worth
finding before forming a theory.

## Boundary, written last

The turn had spent its tool-round budget, and the notice was reporting an
internal limit as an external failure. Both the notice and the missing
truncation magnitude were then fixed from the evidence rather than from the
report.

## What the recollection got right

"Down" was the correct alarm and the wrong diagnosis. A process that took it
literally would have restarted a healthy pod and seen the symptom disappear
until the next deep turn, which is a fix that reports success and changes
nothing.

## See also

- [settling an anomaly](sirens-echo-anomaly-evidence.md) - the order.
- [reviewing a claim](sirens-echo-reviewing-claims.md) - the adjacent question.
