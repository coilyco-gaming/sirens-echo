# The worklog element

A long turn shows what it is doing, one row per tool call, resolving in place:

```
Working on it
> ✅ `eco.get_market`
> 📭 `eco.get_stores`
> ❌ `forgejo.list_issue`
> 🔨 `eco.find_trade`
4 tools, 14 seconds elapsed
```

Direction A of the sirens-echo#111 design. **You can see progress rather than motion**: resolved rows
read as working, where a lone spinner reads as possibly hung, and #137 and #190 were both a live service
looking dead.

**An embed needs `EMBED_LINKS`, missing from every install link this estate published**, so the
permission decides the surface: granted gives the embed, absent gives the stacked notice lines
unchanged. The permission is read rather than assumed, and **a channel that reads as unknown is treated
as granted, because a direct message always reads that way**: a wrong yes costs one refused call, **a
wrong no strands the richer surface for a whole turn**. **A refusal degrades, it never fails** - Discord
answering `50013` routes the turn to the notice lines and latches, so a channel without the grant pays
one rejected call rather than one per beat, **because posting nothing is the silence this element exists
to remove**.

**The glyphs are the reaction vocabulary**: 🔨 invoked, ✅ ok, 📭 empty, ❌ failed, shared with the
reactions and the disclosure footer rather than invented here, **so one state model has three
renderings**. An unresolved row on a stopped element keeps 🔨, carrying its approved meaning exactly,
invoked with the outcome never learned, **because claiming failure there would be a claim the harness
cannot support**.

**The embed is a container, not an exemption.** Every row is a harness notice, the same shape a member
reads outside an embed, which is why the alphabet gained the underscore: **the tool name is the payload,
and `list_issue` sanitized to `list issue` is a name nobody can look up.** Markdown cannot act on an
underscore inside a code span, and the backtick is still stripped. Rows cap at six with a count standing
in for the rest, and **arguments are never shown**, since names are roster-derived and arguments carry
member text.

A delivered answer **deletes** the element, because the answer carries the disclosure footer which
already names the tools, **and two lists of them is what #385 avoided**. Anything else resolves to
`Did not finish` and stays, because **an element that merely vanishes mid-narration is the #137 silence
in a costume**. **A block is not tellable from any other stop**: one wording, no reason carried, and
nothing in the view takes a category, **so the property is structural rather than a convention someone
has to remember** - a progress surface naming the classifier undoes #226 in one line.

**The element does not become the answer.** The design has the embed cleared and the same message filled
with the reply, which routes delivery through the progress message, **bypassing the overflow-attachment
path (#791) and the thread routing**, so it is its own change. **The threshold is unchanged at 5
seconds**: the design table says ~2.5s, but `turnProgressAfter` is an operator knob with the beat and
the thread threshold derived from it, **so halving it halves both**.

**The receipt names a call the way this element did.** The footer rendered the model-facing
`scratchpad__write` while the row above rendered `scratchpad.write`, so a member who watched a scratch
call run looked for it afterwards and found nothing (#900). `ExecutedTool.Label` is the one spelling
both take. **Neither surface filters anything**, and the receipt's collapsed runs state their count.

## Attribution

Reading the record of any effect answers "who asked for this" without inference. **Attribution is
evidence. Authority is enforcement.** A system can have either without the other, and the failure modes
differ: missing authority lets the wrong thing happen, **missing attribution means nobody can find out
that it did**. This records who asked and decides nothing.

**What identifies a principal in the record** is the Discord user ID, stable and opaque, which is what a
durable record wants; a handle is readable and mutable, **so it is display only and is never the
recorded value**. **Attribution does not reach telemetry as a span attribute, deliberately.** The job id
is already promoted on spans and log rows and resolves to a principal through the record, **so the
indirection is sufficient and it keeps a user identifier out of the telemetry store entirely**. That is
the better property rather than merely the cheaper one: **telemetry is exported, retained on someone
else's schedule, and read by more things than the record is**, while `sirens_echo.job.id` is queryable
and means nothing on its own, the join needing the store **where the access question already lives**. A
test asserts that neither a span nor a log row carries the principal, **so this stays true rather than
being a current fact**.

**Retention is unanswered and correctly flagged as not blocking.** A durable record of who asked for
what is exactly what makes attribution useful and exactly what makes it a data question: it needs an
answer before the guild widens beyond a two-party channel, **and it is a policy decision rather than an
engineering one**.

`AttributeJob` returns the requester from the record. `AttributeEffects` resolves a job's applied
effects to the principal that caused them, with the step, its detail, and the terminal state.
`ListByPrincipal` lists a principal's jobs. **No production path calls the first two**, so they are
answerable rather than answered (#620): `AttributeEffects` reads `job.Effects`, which only
`RecordEffect` writes and nothing calls, **so the map is empty for every job that has run**.

**Attribution survives the job.** A job that failed or was cancelled still names its requester and still
appears in that principal's listing, **which is the case worth tracing, so it is the case tested
first**: an attribution that only covers successes is one that disappears exactly when someone needs it.
It does not decide what a principal may cause, and it puts no message content anywhere. The principal
field was kept when this item was deferred, **on the grounds that adding an owner to existing records
later is worse than carrying one from the start**, and that decision is what made this issue small.

## The consult gate

`consult` decides two things: how `cli-guard` dispatches an issue, and what a human sees when they look
for work that needs them. **It was measured wrong in both directions at once**, which is why it has a
document. Written out it is **`autonomy/async-consult`**, one of an exclusive `autonomy/*` set, so
applying it removes `autonomy/headless`, **and a command line has to say the whole thing or it silently
applies nothing**.

**The label is written once, when someone thinks to write it, and never again**, with nothing
reconciling it against the thread afterwards, so it drifts two ways. **An answer arrives and the label
stays**: the work looks blocked, nobody takes it, and the person who filed the question does not come
back because the board tells them it is still waiting - one measured instance sat four hours after being
answered, with its author working other issues the whole time. **A question is asked in prose and no
label is written**: the work looks free, so nobody answers it, **which is the worse direction, because
unlabelled is correctly fail-closed for dispatch and invisible to the human**, leaving the issue both
undispatched and unqueued. Measured on one sweep, **the queue advertised eighteen items when eight were
real, while hiding five**: a reader who trusted it would have been wrong about fifteen of twenty-three.

**Recording a decision removes the label. Asking a human a question adds it.** You are already writing
to the issue at that moment, so each costs one call and nothing else, and a comment ending with a
question for a director or an operator **is a `consult` issue by definition**.
