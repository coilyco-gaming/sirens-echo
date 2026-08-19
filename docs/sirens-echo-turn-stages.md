# Turn stages

A turn's wall clock is attributable to named stages.

## One allowance for the whole turn

`ModelBudget` bounds **one completion**: `Complete` runs `tool_rounds + response_repairs +
budget_raises + 1` model calls, then withdraws the tools and asks for an answer from what it gathered.
**A turn is not one completion.** The content gate, the answer, and the filing check are separate
`Complete` calls each opening that ceiling again, so it multiplied rather than bound.

Measured on `sirens-dowel` over 24h to 2026-08-19: `model.round` never passed **15**, the last round of
a 16-call ceiling, while the heaviest turn ran **36** `model.chat` spans in 235.9s. Its rounds read 0-8
three times and taper to 13 once, so that turn is three completions of 14, 13, and 9 rather than one
long loop, and `mcp.tools.list` sits beside them three times. **Nothing was counting the turn**, which
is why neither a shorter `SIRENS_ECHO_REQUEST_TIMEOUT` nor a smaller `tool_rounds` was the fix: the
clock drops the hard turns, and `tool_rounds` was already being honoured.

`SIRENS_ECHO_TURN_MODEL_CALLS` (`24`, per lane `model_budget.turn_model_calls`) rides the turn context
and is spent by every completion under it. **It gates investigation and never the answer**: under two
calls a tool round no longer fits, so tools are withdrawn and the spent-budget notice appended, logged
as `model.turn.budget.spent`, while each completion still buys its final call. A starved content gate
or filing check fails the turn outright, worse than a turn answering from partial evidence and saying
so. A round costs two calls, so an allowance under two is refused rather than shipped as a silently
toolless lane. Only an ingress turn installs one, so the board, bridge, and rate lane are
unchanged. Count `model.chat` per `trace_id` for a turn's whole spend.

## The settle wait

Between the last `model.response` and `turn.reply.ready` a turn sits under `turn.progress.settle`.
**A member's answer can be held for up to 10 seconds after it is ready**, and whether that trade is
right is a separate question from whether it is visible.

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
