# Turn stages

A turn's wall clock should be attributable to named stages. Two intervals were not, and both of them
were where the failures decide.

## The gap, and what was in it

Between the last `model.response` and `turn.reply.ready` a turn spent seconds under no span at all. One
measured turn spent 9.02 seconds there, 43% of its duration, and a successful one spent 4.34
(sirens-echo#652). **It is the settle wait**, and `turn.progress.settle` now covers it. Worth knowing
rather than only worth measuring: **a member's answer can be held for up to 10 seconds after it is
ready**, and whether that trade is right is a separate question from whether it is visible.

## Which check refused a reply

`response.validate` carries `response.check`, naming the check that refused, or `none` when the reply
passed. **The value is present on every turn, because absence of an attribute is not something a reader
should have to interpret.** The closed set, in the order they run: `parse`, `tool_call_markup`,
`grounding.invented_channel`, `grounding.claimed_action`, `grounding.tracker_action`,
`grounding.continuing_work`, `self_attributed_claim`, `identifier_disclosure`, `identity_claim`,
`response_style`. **Order is the contract**, so the checks are a slice rather than a chain of
conditions, and the first to refuse is the one named. **Grounding is four rules, so it is four values**,
because a refusal rate over the family could not separate a model inventing channels from one claiming
actions it did not take, which are different problems with different fixes. The span also carries
`response.check.reason`, the sentence the validator wrote.

**A named check no longer always means a discarded turn**: a non-zero `response.redacted.blocks`,
present on every turn, is a reply that was cut and sent rather than thrown away. Before this, a rejected
reply produced `turn.stage.failed` with `error_type: model_failed`, **which reads as the backend
failing** when it was the harness refusing the model's output (#651, where two correct answers were
discarded and reported as a backend outage).

The completion layer runs these checks too, before the authoritative pass, **so a refusal reaches the
model that can still fix the clause**. `model.response.repair` records what it refused rather than only
that it happened, and `model.response.refused` the reason a turn gave up. That path reports
`stage=model`, **true about the code and false about the world**, which is why a reply the repair budget
could not fix is handed back to the pass above rather than failed there. **Naming a check does not
change any verdict**: whether a rule is right, and whether exhausting the repair path should report an
outage, are #651 and #396.

`turn.reply.delivered` is emitted when a send returns, so a delivered reply can be counted. Previously
only the failure was recorded, so delivery had to be inferred from the absence of an error, **and an
investigation into replies that never arrived could not tell a silent success from a silent loss**.

## Turn identifiers

A member handed a trace id can only use it if the turn it names can be placed, so the turn span carries
Discord's own identifiers. On a guild turn: `discord.user.id`, `discord.guild.id`, `discord.channel.id`,
`discord.thread.id` (only inside a thread), and `messaging.message.id`, the member's message rather than
the reply. **The keys match the ones `discord.receive` and the send span use, and so do the values,
because all three read one resolved location**: matching keys carrying disagreeing values would make an
operator's query partial without looking partial (#348).

**Thread and channel are not the same field.** Discord models a thread as a channel, so a naive mapping
reports the thread id as the channel id **and every thread turn disappears from a query for the channel
it hangs under**. Resolution reads cached gateway state and never calls the API, and a channel not in
the cache contributes no thread id: **absent beats blank, because an empty thread id reads as a thread
nobody can find.**

**The account id is here by reversal.** The code previously said, beside the span, that the requester
was deliberately not an attribute because an account id is not operational telemetry. That was
overturned on purpose in issue 337 by the director, because **an account id in a private telemetry
backend and a handle in a public reply are different objects**. The first answers "show me every turn
this member had trouble with" and cannot be reconstructed from a job record when the trouble is a turn;
the second is still refused by the reply validators. Prompt, model, tool, and reply bodies stay out of
telemetry, as does anything member-visible: a display name, a nickname, a handle. **Identifiers go to
the backend and never into what a member reads**, and a direct message contributes no attributes at all.
`SpanAttributes` is the single place the account id is added and one test asserts it is present, **so
removing it means deleting that assertion in the same commit**, deliberately, so the removal is a
decision someone records rather than a line that quietly rots.

## What can reach a turn's prompt

Every input to a turn, so a claim about context bleed can be checked against a list rather than argued
(issue 265). Three vary per turn: **transcript history** and the **current message**, both from the
caller, and **tool results**, from whatever the roster serves mid-turn. Three do not: the **principal**
from deployment, the **composed bundle** from the image or its placeholder, and the **skillpack and
local policy** from the build. **Only two of those carry member content, and that is the whole
surface**: `BuildSystemPrompt` and `BuildTurnPrompt` are functions of their arguments with no package
state, no file reads, and no environment lookups. `turnisolation_test.go` asserts, mutation-checked
against a planted accumulator, that a second turn's prompt carries nothing from the first, that the same
inputs produce the same prompt however many turns ran between them, and that the turn context holds
exactly the entries the caller supplied. **The first is the one that matters**, because a bleed inside
this repository would arrive as a cache or an accumulator added later for a good reason.

They cannot catch a caller that supplies the wrong window, a tool returning content from another
surface, or **anything stateful added later**: a scratchpad is the live example, and any store that
outlives a turn belongs in that list with its own reasoning.

## The turn event is not the send event

Two events carry `discord_failure` and only one is about Discord. **Reading the wrong one is what made
an 18 percent delivery failure rate out of a population that had barely any delivery failures in it.**
`discord.turn.failed` fires when the turn returned any error and classified it the way a failed send is
classified, so a model stage failure fell through to the catch-all and reported `no_response`, **the
value that means the gateway never answered**. In one 24 hour window, thirteen of fourteen classified
rows were turns that failed at a stage and none had reached a send.

The turn event now classifies a Discord verdict only where one exists: the reply send is marked as it
returns and everything else reports `discord_failure: not_attempted`, **so the two populations split in
a group-by instead of merging**. A run of `not_attempted` says the turns died before delivery and points
at the model stage, and a run of `no_response` now genuinely means the gateway went quiet.

Count delivery failures on `discord.reply.failed` **and nowhere else**. `turn.reply.undelivered` fires
when the apology for a failed send also failed, so a member who got neither the answer nor the notice is
in that one. **Reading a window that straddles the change**: `not_attempted` is newer than the pods that
predate it, so a row carrying no `discord_failure` at all is an old image rather than a turn that
classified to nothing.
