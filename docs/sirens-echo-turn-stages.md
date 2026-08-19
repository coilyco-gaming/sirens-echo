# Turn stages

A turn's wall clock is attributable to named stages.

## The settle wait

Between the last `model.response` and `turn.reply.ready` a turn sits under `turn.progress.settle`.
**A member's answer can be held for just under one full beat after it is ready**, which is
`turnProgressEvery`, twice `SIRENS_ECHO_PROGRESS_AFTER`. At the packaged 10s wait that is a hold of
**up to 20 seconds**, not ten: `settleDelay` returns `turnProgressEvery - remainder`, so the ceiling
is the beat rather than the wait. This page said ten, and the telemetry disagreed with it in the
open: measured on `sirens-dowel` over 24h, `turn.progress.settle` ran p50 15.97s against p95
**19.9997s**, which is the 20s beat and could not have been the 10s wait.

**Half of the number is not a rounding error when the number is the argument.** Whether holding a
finished answer to protect the cadence of a line that is deleted when the answer lands is the right
trade is a separate question from whether it is visible, and it is a question someone decides by
reading this sentence. Ordinary turns are where it bites rather than slow ones: three median
`sirens-dowel` turns spent 9.5s of 22.3s and 20.0s of 35.3s in the settle, against 0.6s and 1.1s in
tool calls.

## Which check refused a reply

`response.validate` carries `response.check`, naming the check that refused, or `none` when the reply
passed. **The value is present on every turn**, because absence of an attribute is not something a
reader should have to interpret. The closed set, in the order they run: `parse`, `tool_call_markup`,
`grounding.invented_channel`, `grounding.claimed_action`, `grounding.tracker_action`,
`grounding.continuing_work`, `self_attributed_claim`, `identifier_disclosure`, `identity_claim`,
`response_style`.

**Order is the contract**, so the checks are a slice rather than a chain of conditions, and the first to
refuse is the one named. **Grounding is four rules, so it is four values**: a refusal rate over the
family could not separate a model inventing channels from one claiming actions it did not take, which
are different problems with different fixes. The span also carries `response.check.reason`, the sentence
the validator wrote.

**A named check does not always mean a discarded turn.** A non-zero `response.redacted.blocks`, present
on every turn, is a reply that was cut and sent rather than thrown away.

The completion layer runs these checks too, before the authoritative pass, **so a refusal reaches the
model that can still fix the clause**. `model.response.repair` records what it refused rather than only
that it happened, and `model.response.refused` the reason a turn gave up. That path reports
`stage=model`, **true about the code and false about the world**, which is why a reply the repair budget
could not fix is handed back to the pass above rather than failed there. Naming a check changes no
verdict.

`turn.reply.delivered` is emitted when a send returns, so a delivered reply can be counted rather than
inferred from the absence of an error.

## Turn identifiers

A member handed a trace id can only use it if the turn it names can be placed, so the turn span carries
Discord's own identifiers. On a guild turn: `discord.user.id`, `discord.guild.id`, `discord.channel.id`,
`discord.thread.id` (only inside a thread), and `messaging.message.id`, the member's message rather than
the reply. **The keys match the ones `discord.receive` and the send span use, and so do the values,
because all three read one resolved location**: matching keys carrying disagreeing values would make an
operator's query partial without looking partial.

**Thread and channel are not the same field.** Discord models a thread as a channel, so a naive mapping
reports the thread id as the channel id **and every thread turn disappears from a query for the channel
it hangs under**. Resolution reads cached gateway state and never calls the API, and a channel not in
the cache contributes no thread id: **absent beats blank, because an empty thread id reads as a thread
nobody can find.**

**The account id is an attribute on purpose.** An account id in a private telemetry backend and a handle
in a public reply are different objects: the first answers "show me every turn this member had trouble
with" and cannot be reconstructed from a job record when the trouble is a turn, and the second is still
refused by the reply validators. Prompt, model, tool, and reply bodies stay out of telemetry, as does
anything member-visible: a display name, a nickname, a handle. **Identifiers go to the backend and never
into what a member reads**, and a direct message contributes no attributes at all. `SpanAttributes` is
the single place the account id is added and one test asserts it is present, **so removing it means
deleting that assertion in the same commit**.

## What can reach a turn's prompt

Every input to a turn, so a claim about context bleed can be checked against a list rather than argued.
Three vary per turn: **transcript history** and the **current message**, both from the caller, and
**tool results**, from whatever the roster serves mid-turn. Three do not: the **principal** from
deployment, the **composed bundle** from the image or its placeholder, and the **skillpack and local
policy** from the build.

**Only two of those carry member content, and that is the whole surface.** `BuildSystemPrompt` and
`BuildTurnPrompt` are functions of their arguments with no package state, no file reads, and no
environment lookups. `turnisolation_test.go` asserts, mutation-checked against a planted accumulator,
that a second turn's prompt carries nothing from the first, that the same inputs produce the same prompt
however many turns ran between them, and that the turn context holds exactly the entries the caller
supplied. **The first is the one that matters**, because a bleed inside this repository would arrive as
a cache or an accumulator added later for a good reason.

Those tests cannot catch a caller that supplies the wrong window, a tool returning content from another
surface, or **anything stateful added later**. A scratchpad is the live example, and any store that
outlives a turn belongs in that list with its own reasoning.

## The turn event is not the send event

Two events carry `discord_failure` and only one is about Discord.

`discord.turn.failed` classifies a Discord verdict only where one exists: the reply send is marked as it
returns and everything else reports `discord_failure: not_attempted`, **so the two populations split in
a group-by instead of merging**. A run of `not_attempted` says the turns died before delivery and points
at the model stage, and a run of `no_response` genuinely means the gateway went quiet.

Count delivery failures on `discord.reply.failed` **and nowhere else**. `turn.reply.undelivered` fires
when the apology for a failed send also failed, so a member who got neither the answer nor the notice is
in that one. **Reading a window that straddles the change**: a row carrying no `discord_failure` at all
is an old image rather than a turn that classified to nothing.
