# Threads

A turn that took long enough to announce itself puts its answer somewhere of
its own. This document says when that happens, what it costs, and what it must
never cost.

## When

Two conditions, both required:

- the turn posted a progress line, so something in the channel points at the
  thread
- the turn ran past `turnLongReplyAfter`, the wait plus two beats

The second alone is not enough. A turn that never posted a line has nothing in
the channel referring to a thread, and a member would be left with a question
and no visible answer.

## Carry or announce is not a choice here

The obvious worry about moving a reply into a fresh thread is that the channel
then shows a question and nothing else. That worry is already answered by code
that predates this: a long turn posts a progress line in the channel, and that
line is the announcement. The reply lands in the thread, the channel keeps the
line.

This also makes the guild's hide-after setting cheap. A thread that auto-hides
takes nothing with it that was not already duplicated in the channel, because
the channel-side artifact was never the answer.

## The name comes from the member

Discord requires a thread to have a name, and a name is member-facing. It is
derived from the member's own message rather than authored: letters, digits
and spaces are kept, everything else is dropped, and the result is truncated
to Discord's hundred-rune cap. A message that is all mention and punctuation
falls back to a fixed phrase, because a thread cannot be created without a
name at all.

Dropped, not summarised. Summarising would be writing a member a title.

## What it must never cost

**A thread that cannot be made must not cost a member their reply.** No
permission, a channel type that cannot hold threads, an API failure, a turn
already inside a thread: every one of those returns "no thread" rather than an
error, and the reply goes to the channel exactly as it did before. A feature
that eats answers is worse than no feature.

That is why the thread decision returns a channel id and a boolean rather than
an error. There is no failure to propagate, because no failure here is worth
failing a turn over.

## Nesting

A turn already inside a thread does not start another. Discord does not permit
it, and it would be the wrong shape if it did. The check reads cached gateway
state and makes no API call.

## What is not here

Threads for jobs. `BindJobToThread` and `ResolveThreadJob` exist and are still
unwired, which is a separate piece of work: a job outlives its turn, so the
question of which thread it owns has a different answer.

## See also

- [progress cadence](sirens-echo-progress-cadence.md) - where the window comes
  from.
- [tuning numbers](sirens-echo-tuning.md) - the window itself.
