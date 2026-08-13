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

## The name says what the thread is for

The title summarises the member's intent: *how much does it cost to build a log
house* becomes something like *log house pricing*.

An earlier version refused to summarise, on the grounds that summarising is
writing a member a title. That was overruled deliberately in issue 461, and the
reversal is recorded rather than quietly applied.

A summary needs the model, so this is one short extra completion, proportionate
because a thread only happens on a turn that already ran past the long-reply
window: the expensive minority by construction rather than every turn.

**It degrades rather than fails.** If the title call errors, times out, or
returns nothing usable, the thread is still created with the mechanically
derived name. A feature that could cost a member their thread over a title
would be a bad trade.

The summary takes the same cleaning as a derived name, so it cannot introduce
markup a member's own message could not.

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
