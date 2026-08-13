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

An anomaly settled in this order, including the false trail it produced and the
number that ended it. See [the worked
example](sirens-echo-anomaly-worked-example.md).

Reviewing someone else's finding is the adjacent question, covered in
[reviewing a claim](sirens-echo-reviewing-claims.md).

## Your own account is an account

The artefact beats the account, and an investigating agent produces an account
throughout. Three from one night, each a real command run correctly against a
question adjacent to the one asked: a grep for `Reply` found nothing because
the field is `Text`, an unrendered index returned a shell that read as links
having drifted, and a rule sought in the wrong file read as a regression.

## Name what you could not check

Cloudflare refused every automated client, a credential request was denied, and
silence could not be told from absence of work. Saying so stops a reader
treating an unchecked thing as a checked one.

## What this is not

It is not a check. Whether an investigation followed this order is a judgement
about the investigation, not a property of a reply, and there is no mechanical
form of it. It is a habit, and the incident above is the evidence that the
habit produces a different answer than the narration does.
