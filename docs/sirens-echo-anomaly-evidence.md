# Settling an anomaly

Human recollection is the lead. Traces and configuration are the settlement.
Model narration is neither, and it is the trail most likely to be followed.

## The order

1. An anomaly is reported. Treat the report as the strongest available signal
   that something is wrong, and as no evidence at all about what.
2. Collect the recollection while it is fresh. What was asked, what came back,
   what looked wrong. Do not correct it against a log yet.
3. Settle it from traces, logs, and configuration. Values a process emitted or
   was started with, not summaries of them.
4. Write the boundary last, from what the evidence supports rather than from
   what the recollection suggested.

## Why the order is not arbitrary

A person notices the event that does not fit. That is what makes recollection
the right lead: an anomaly is by definition out of distribution, and noticing
out-of-distribution things is the thing people do better than a model.

A model summarising an event it took part in produces a fluent, plausible
account with the anomaly smoothed away, because smoothing is what next-token
prediction is good at. **The narration reads most confident exactly where it is
least reliable.** So it is not weaker evidence than a trace. In this one case
it is actively misleading, and it arrives already shaped like a conclusion.

Keep it out of the resolution path entirely. Use it as a symptom to explain,
never as an account to check the traces against.

## The worked example

2026-08-13, recorded because it followed this order before it was written down.
See [issue 258](https://forgejo.coilysiren.me/coilyco-gaming/sirens-echo/issues/258).

**Lead.** Kai reported that Sirens Deep was down. Correct that something was
wrong, and wrong about what: the service was answering.

**The false trail.** The service's own account was `model backend unavailable,
retry shortly`. Taken as the explanation, that sends an operator to the
inference tier to debug a backend that was healthy.

**Settlement.** Ops reconstructed nine model request and response pairs, every
one carrying `status: 200`, then a turn failure emitted 74 to 93 microseconds
after the last good response. A sub-100 microsecond gap is in-process handling.
It cannot be a network call, a timeout, or an upstream outage.

**Boundary, written last.** The turn had spent its tool-round budget, and the
notice was reporting an internal limit as an external failure. Both the notice
and the missing truncation magnitude were then fixed from the evidence rather
than from the report.

Note what the recollection got right and wrong. "Down" was the correct alarm
and the wrong diagnosis, and a process that took it literally would have
restarted a healthy pod and seen the symptom disappear until the next deep turn.

## What this is not

It is not a check. Whether an investigation followed this order is a judgement
about the investigation, not a property of a reply, and there is no mechanical
form of it. It is a habit, and the incident above is the evidence that the
habit produces a different answer than the narration does.
