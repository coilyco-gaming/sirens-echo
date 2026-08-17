# Notices and delivery failures

The phrases a member reads when a turn does not land, and what the service records about why.

## Harness notices

A notice is a member-facing string this service wrote itself. **It is never model output, and the
rendered shape is what tells the two apart**: one blockquoted code span, `> \`rate limit exceeded\``,
the blockquote and code span both literal and both required.

The phrase inside is the short technical form a semi-technical member recognizes. **A notice is not a
sentence**: `http 500 internal server error` is a notice, while "the http server had an internal server
error" is prose, which is what a model reply sounds like, **which is the confusion the format exists to
prevent**. The alphabet is lowercase letters, digits, spaces, and `, . / - _`, everything else stripped,
**so no phrase can close the code span early or span two lines**. `internal/community/notice.go` holds
every phrase, so a new condition adds one there rather than a string at the call site, and
`harnessNotice` is the only constructor, falling back to a fixed phrase when nothing usable survives.

**A failed turn always replies**, the class chosen from the stage and the cause, because the member's
next useful move differs per class: `turn timed out, retry shortly`, `tool call failed`, `channel
history unavailable`, `model backend unavailable, retry shortly`, `reply blocked by response check,
rephrase`, or `turn failed`. **A deadline and a tool failure outrank the stage**, since both name the
surface to stop waiting on more precisely. The notice is sent on a context detached from the turn
deadline, because **a turn that failed by expiring has no budget left to say so otherwise**, which is
how the slowest failures used to end as silence. No model round trip is involved, so **the error path
cannot inherit a model failure it was written to report**.

**A notice carries a condition class and nothing else**: no model output, prompt text, tool payloads,
MCP endpoints, stack detail, member identifiers, or internal error strings. That boundary is why the
phrases are a closed set, since **a formatted upstream error would put an arbitrary internal string in
front of a member**.

**A failed turn cites itself**, carrying a second notice line naming its trace, so a member's screenshot
is a query rather than a report. It reads `trace id` rather than `trace ID:` because both lines take the
notice alphabet. Outside a span the line is omitted entirely, since **a blank identifier reads as a
defect rather than an absence**, and **a successful reply never carries one**.

## Why a ready reply did not land

A turn that fails after the reply is composed has already spent its completions and its MCP calls.
**The work was done and paid for, then dropped at the last step**, invisible to every instrument that
watches the model.

`discord.turn.failed` carried one field, `error_type: turn_failed`, so rate limiting, over-length, a
missing permission, and a dropped gateway looked identical: **a count without a cause**. `discord.reply.failed` now records the send itself, with `discord_failure`
separating whether Discord answered at all, `discord_status` separating rate limiting from permissions
from length, `discord_code` separating two failures sharing one status, and `reply_bytes` proving the
length case. **`no_response` is a classification rather than a gap**, a dropped gateway producing no
HTTP exchange, and `abandoned` is separate because **our own budget ending a send is not an outage**.
**No channel, no member, no reply body**: status and code separate every cause without an identifier.

**A stage is not a cause.** `error_type` is the stage, so a timeout and a backend outage at the model
stage collapse into one value while showing a member two different notices, countable only through the
notice string, which is prose rather than a label. `failure_cause` is a closed set - `shutdown`,
`timeout`, `tool_failed`, `rounds_spent`, `reply_refused`, `budget_spent`, `stage_failed` - **derived in
the same order the notice is chosen so the label and the phrase a member reads cannot disagree**.

**The member is told, once.** A composed reply that fails to send used to end the turn silently, which
is indistinguishable from being ignored, so the turn marks itself failed and attempts one short notice,
`reply could not be delivered, retry shortly`. **One attempt, never a retry**, because a loop would turn
one dropped reply into a flood against the transport that just refused. The second send is worth trying
because **the failure classes differ in size**: a reply refused for length succeeds as a short notice,
while a permissions failure costs one call and fails again.

**When the notice itself cannot be sent**, a member who waited already has an acknowledgement in the
channel: the progress line. Deleting it after a notice fails ends the turn with less than they had,
**and dead air is the worst outcome this service has**. So the notice is carried by that line instead,
an edit being a different call against a message that already exists, and **a claimed line is never
deleted even when the edit fails too**.

## Why a check refused a reply

A refused reply names the check that refused it, and now says what that check saw. `ValidateGrounding`
produces `model invented channel #general`, and that sentence was generated and thrown away, so reaching
it took four steps and a source read (#795). `response.validate` carries `response.check.reason` and
`response.check.refused` logs the same sentence under `refused`.

**Not every refusal gates.** A rule about how an answer *reads* records and the reply still ships:
`response_style` and `tool_call_markup` (#651). **An unlisted rule gates, so a new one fails closed**,
and a shipped reply carries `response.check.shipped`.

**The reason is not an exception field.** `MarkSpanError` still takes only a catalog code, so
`exception.type`, `exception.message`, `error.stage`, `error.outcome`, and the span status stay the
fixed cataloged wording, **grouping and alert filters unmoved**. The reason is an ordinary span
attribute, so adding a rule needs no catalog entry and no reviewed cardinality increase.

**Every refusal sentence is service-authored except one.** `grounding.invented_channel` names the
`#token` the model wrote, deliberately, because **the token separates a channel missing from supplied
context, which is a prompt problem, from a pure hallucination, which is not**. It is bounded twice:
`channelPattern` constrains the shape, and 64 runes cap it so one long hallucination cannot enlarge every
refusal record. **The identifier check stays the counterexample**: its error names the class and never
the value, because the value is what is being kept out of a log.

## Exit paths

The entrypoint's own failures are JSON on stdout carrying a populated `level`, in the same shape as
every other record, **so ingest promotes a severity from them and a severity alert can match the process
dying**. `startup.config.failed` and `startup.telemetry.failed` land before `Telemetry` exists and use a
standalone startup logger with the same handler configuration; `startup.agent.failed` and `run.failed`
go through `Telemetry.Error` and are trace-correlated; all four exit, and
`shutdown.telemetry.failed` does not. Each carries the cause under `error`.

**The stdlib `log` package must not reach this binary.** `log.Fatalf` writes unstructured text to
stderr, and the ingest pipeline parses a JSON body, so an unstructured line fails the parse, produces no
`level` attribute, and reaches neither the severity parser nor an alert filtering on `level`. **That
makes a crash the one event no severity alert can see.** Known gap: a fatal path exits without running
the deferred telemetry shutdown, so a crash does not flush its pending traces and metrics, though the
log record still reaches stdout because the handler writes synchronously. Recorded rather than fixed.
