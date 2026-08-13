# Recognising a counterpart agent

Deep can tell whether the account it is answering is an agent or a person, and
the determination reaches the turn.

## Grounded, not inferred

Discord marks bot accounts, and that flag is ground truth costing nothing. An
agent that guessed from writing style would be a worse version of the problem:
anyone can write "I am an agent", and a member who does must not become one.

So recognition reads `Author.Bot` and nothing else. A prose claim moves nothing,
which is asserted in the tests rather than assumed.

## Admission comes first

Bot authors were rejected outright, so a counterpart never reached a turn at
all. Recognition therefore required admitting them, which widens the summon
surface, so it is opt-in by name.

`agents.allow` in the access policy lists the counterpart accounts answered.
Empty answers none, which is the shipped posture: no agent arrives by upgrading.
A bot not on the list is refused exactly as before, and Deep never answers
itself regardless.

A named counterpart is **admitted, not trusted**. It passes the same channel and
guild gates a member does, and being an agent grants nothing.

## The exchange is bounded

Two agents in one channel that each answer the other is a runaway. Recognition
is what makes the bound expressible, so the bound is here:

* consecutive agent-authored turns in one channel are capped
* a human turn resets the run, because a person joining ends the loop
* a capped exchange does not resume by waiting, so the cap is not a speed limit
* a genuinely quiet channel forgets, so a later exchange is a fresh one
* the bound is per channel, so one pair cannot silence another

The check runs before admission spends anything, which is the cheapest place for
it.

## What reaches the model

An agent-authored line is marked in the turn context:

```
- Sirens Echo (an agent, not a person): reporting in

The request that follows is from Sirens Echo (an agent, not a person).
```

A human turn gains no marking at all. An annotation that is sometimes absent is
more useful than one that is sometimes wrong, and the same reasoning applies
here as to the job id on spans.

The member's message itself is untouched, per the final-user-turn rule.

## What deliberately does not change

Behaviour. Deep recognises a counterpart and is told so, and nothing else about
the turn differs: no disclosure line, no register change, no altered trust.

That is the smallest slice that satisfies the recognition axis, and the
behaviour question is Kai's to settle. Whichever answer it gets, the
determination it needs is now available to the turn.

## Interaction with execution

An admitted counterpart widens the requester set beyond one account, so
`CheckExecutionAdmission` refuses execution while one is allowed. Recognition
and executing jobs are not enabled together by accident.

See [access](sirens-echo-access.md) and [the identity
eval](sirens-echo-recognition.md).

## A refused agent is visible, not only counted

Both refusals log without an identifier, since a counter never says how often
in a row: `discord.agent.ignored` and `discord.exchange.bounded`.
