# What decides before the model does

Read this when asked how the service decides what to answer, why a message was
refused, or what governs replies. It describes the layers **outside** the model,
so a self-description can cover them from a source rather than from a guess.

Everything below runs in the harness. The model does not run it, cannot see it
decide, and cannot override it.

## The content gate

A closed taxonomy in `agent/content-classes.yaml` classifies what a member is
asking for. Its shape matters more than its contents:

* **Allow by default.** Denied classes are the exception, and the list also
  enumerates the allowed ones so that an ordinary request has somewhere to go
  rather than being forced into a refusal bucket.
* **A `sensitive` class changes the shape of a refusal, not the verdict.** An
  ordinary block names its reason. A sensitive block gives a generic redirect
  naming no category, because saying which rule fired tells someone what to
  avoid saying next time.
* **Sensitive wins ties.** A request matching both resolves to the sensitive
  one, so the ordinary category is not named out loud alongside it.
* **Every block is visible.** Silent no-reply was rejected, so a refusal is
  never mistakable for an outage.
* **A block is one sentence**, for the reason in the refusal rules: a refusal
  that argues its own case is the longest thing the service says.

So a refusal that gives no reason is not evasion and not a fault. It is the
sensitive branch working, and describing it that way is accurate.

## Whether a message arrives at all

A message reaches the model only after the harness decides it was addressed to
this service. An explicit mention qualifies. So does a reply to one of its own
messages, and a message in a thread it opened. Anything else in a channel it can
read is not answered.

**Another bot is refused unless its id is allowlisted**, so the service does not
talk to arbitrary automation, and its own messages are dropped before any of
this so it cannot summon itself.

Rate limits apply per requester, per channel, and globally, and a bounded number
of requests may be in flight at once. Past those bounds a request is declined
rather than queued, because nothing here runs work between replies.

## What this does not cover

**The runtime is not observable from inside a turn.** Which model served the
request, how long it took, what the logs or metrics say, whether the service is
healthy: none of that is visible here, and no file makes it visible. Those
questions go to an operator, and saying so is the correct answer rather than a
deflection.

Where this file and the running harness disagree, **the harness is right and
this file is stale**. It is a description maintained by hand, so treat a
confident detail here as weaker evidence than an observed refusal.
