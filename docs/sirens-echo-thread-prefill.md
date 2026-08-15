# Reading a whole thread

A turn inside a thread got the same partial window a channel turn gets, so a
thread longer than that window was answered from its own tail. The earlier half
of the conversation was not in the prompt and nothing said so.

## Every thread, with nothing to switch on

Being in a thread is the condition. There is no toggle and no opt-in list.

A per-channel gate was specified and Kai removed it: a thread is a bounded
conversation someone deliberately opened, so reading it whole is what the
feature means everywhere rather than a per-channel preference. The context
budget and the walk bound below are what hold the cost down, not a switch.

## Overflow drops the oldest first

When the thread exceeds the context budget the oldest messages drop until it
fits, and the reply says so. Kai decided that over three alternatives:

* **Silent truncation.** A wrong answer from missing context would look
  identical to any other wrong answer.
* **Falling back to the partial window.** The feature would quietly stop
  applying in exactly the long threads it was built for.
* **Summarising the older half.** A model call and a new failure mode on every
  turn.

The annotation is a service-authored suffix like the tool receipt, so it is
appended after the reply checks and contends for the send budget with the
others. It sits ahead of the receipt in the preference order: a note about
missing context outranks a record of what ran, because the last suffix is the
first one cut.

## The count is exact, or says it is not

The walk is bounded, so a pathological thread costs a known number of Discord
calls rather than an unknown one. A thread longer than the walk is still
truncated and still annotated, and the annotation says `at least` before the
length, because at that point the runtime knows a floor rather than the thread.

An absent hedge is therefore a claim: a plain count means the walk reached the
start of the thread. A thread whose newest messages all fit the budget is still
annotated when the walk stopped short, because nothing going over budget is not
the same as having read the whole thread.

## What it costs

A whole-thread read is one Discord call per hundred messages instead of one
call, and it applies to every threaded turn. `history.thread.read` and
`history.thread.dropped` land on the `community.history` span, so what real
threads produce is readable from the first day this runs rather than estimated.

sirens-echo#750 makes Echo-owned threads permanently summonable, which raises
the number of turns taken inside threads and therefore the cost of this. Land
them in either order and measure prefill size after both.

## What does not change

Outside a thread the prefill is the same partial window, read the same way and
bounded by the same `max_context_messages`. A turn that drops nothing adds
nothing to the reply.

## See also

* [Reply assembly](sirens-echo-reply-assembly.md) - what shares the budget.
* [Threads](sirens-echo-threads.md) - when a reply gets one.
