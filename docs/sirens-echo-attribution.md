# Attribution

Reading the record of any effect answers "who asked for this" without inference.

Attribution is evidence. Authority is enforcement. A system can have either
without the other, and the failure modes differ: missing authority lets the
wrong thing happen, missing attribution means nobody can find out that it did.
This records who asked and decides nothing.

## The three design questions, answered

**What identifies a principal in the record?** The Discord user ID. It is stable
and opaque, which is what a durable record wants. A handle is readable and
mutable, so it is display only and is never the recorded value.

**Does attribution reach telemetry as a span attribute?** No, deliberately.

The job id is already a promoted attribute on spans and on log rows. A job id
resolves to a principal through the record, so the indirection is sufficient and
it keeps a user identifier out of the telemetry store entirely.

That is the better property, not merely the cheaper one. Telemetry is exported,
retained on someone else's schedule, and read by more things than the record is.
`sirens_echo.job.id` is queryable and means nothing on its own; the join needs
the store, which is where the access question already lives. A test asserts that
neither a span nor a log row carries the principal, so this stays true rather
than being a current fact.

**Retention.** Unanswered, and correctly flagged as not blocking. A durable
record of who asked for what is exactly what makes attribution useful and
exactly what makes it a data question. It needs an answer before the guild
widens beyond a two-party channel, and it is a policy decision rather than an
engineering one.

## What is attributable

* **Any job.** `AttributeJob` returns the requester from the record. No
  inference, no trace walking.
* **Any effect.** `AttributeEffects` resolves a job's applied effects to the
  principal that caused them, with the step, its detail, and the job's terminal
  state.
* **A principal's history.** `ListByPrincipal` lists their jobs.

## Attribution survives the job

A job that failed or was cancelled still names its requester, and still appears
in that principal's listing. That is the case worth tracing, so it is the case
tested first: an attribution that only covers successes is one that disappears
exactly when someone needs it.

## What this does not do

It does not decide what a principal may cause. That is per-requester authority,
filed separately, and nothing here grants or denies anything.

It does not put message content anywhere. A principal identifier is not message
content, and the telemetry body-safety contract is unaffected.

Items 7 and 8, authorization beyond admission and approval gates, remain out of
scope.

## Why the record already carried this

The principal field was kept when this item was deferred, on the grounds that
adding an owner to existing records later is worse than carrying one from the
start. That decision is what made this issue small: the field was already there
and already populated, and this makes the listing trustworthy rather than merely
present.

See [the job record](sirens-echo-jobs.md) and
[job telemetry](sirens-echo-jobs-telemetry.md).
