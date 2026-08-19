# Reply progress and reactions

A long turn says what it is doing. A short one says nothing, because **a progress line for a two-second
reply is noise.**

## The line

Nothing for the first ten seconds. After that, one line in the harness notice format that edits in place
as the turn moves: `reading recent messages`, `thinking...`, `calling a tool`, `checking the reply`,
and 📖 `consulting the catalogue...` when the round is a skill read. A
turn that sits in one stage grows a clock line per beat (`still thinking 19 seconds...`). **The line is
removed when the reply lands**, because the reply, or the failure notice, is the turn's real answer.

It uses the harness notice format rather than an embed, because that format is fixed for every
harness-generated message and **an embed while failures and cooldowns are blockquoted code spans would
read as two bots**. Restyling is a contained change to `TurnProgressSink`.

* **Threshold** - nothing is posted until a turn has run long enough to be worth narrating, so **the
  fast path makes no Discord calls at all**.
* **Edit rate** - edits are bounded and a repeated stage is not re-sent. **The column bounds height not
  duration**, advancing in place once full, and **the wait narration doubles its interval per edit**,
  never going quiet.
* **Advisory** - a failed post or edit is dropped rather than failing the turn, and a line arriving
  after the reply is deleted.
* **Mention safety** - empty allowed mentions, as everywhere the harness speaks.
* **Discord only** - HTTP and MCP answer synchronously, so the non-Discord path is a nil progress every
  method accepts.

**Stage transitions alone are not enough.** A turn changes stage twice in its first moments and then sits
in one stage for as long as the model takes, so a line posted only on a transition lands just before the
reply. A watcher therefore ticks alongside the turn and posts the current stage once the threshold
passes, whether or not anything changed. Job progress is a separate mechanism with the same shape,
because **a job's origin outlives its turn**.

## Cadence

**The line starts a grid, and everything later lands on it.** The line posts at ten seconds and every
message after releases on a twenty second grid measured from that post: ready at 10.1s means posted at
30s, ready at 30.1s means 50s. Without it, a turn just over the threshold posts a line and answers a
moment later, so the line vanishes before it is readable.

**Only the ten is written down.** The beat is twice the wait and the long-reply window is the wait plus
two beats, so one number moves all three, and a test pins both the derivation and today's values. The
grid does not stop: a turn still running at the tenth beat waits for the eleventh, so the hold is at most
one window and averages half of one however long the turn runs. **Landing exactly on a beat is on time**,
since rounding a punctual reply up to a whole extra window would be the cadence working against the
member.

**An unnarrated turn is never held**, which is what keeps an ordinary reply fast: a reply before ten
seconds posts no line, so there is no grid to wait for, and a cancelled turn stops waiting rather than
sitting on the answer. **The failure path holds too**, reaching the line through the turn context, since
a notice replacing a just-posted line churns as much as a reply does. A dead turn can therefore show a
stale line for up to one window, and **a notice that jumped the grid would make failure the one thing
that answers instantly**. Edits ride the same beat, with the post counting as the first.

**Every sink call is recorded.** Post, edit, and delete record `discord.progress.posted` or
`discord.progress.failed`, so a refused post is visible without failing a turn.

## Reactions as harness state

A reaction is applied to the member's own message and reports what the harness is doing. **It is not
model output, so it never passes reply validation**, and the neutral contract governs words, which a
reaction has none of. One is an answer ([phrases](sirens-echo-phrases.md)).

* **eyes** - accepted at harness level, as the turn starts, before any model call.
* **hammer** - the turn called a tool, in the tool round.
* **warning** - the turn produced no reply, or gave up waiting for a slot.
* **no entry** - refused at a boundary, on the admission denial.

**A turn shed for load and a message turned away at a boundary do not share a mark.** One is a service
that could not answer and the other is a service that would not, and a member reading the channel is
entitled to tell them apart.

**The accepted mark earns its place** by landing before the first model call, so a turn that then dies
silently is still visible: the message carries a mark and never gets an answer. During a model outage
that turns "the bot has gone quiet and nobody knows why" into a signal every member can see, at no
alerting cost. **A reaction can never fail a turn**: every failure is swallowed, the likeliest cause
being the bot identity lacking `ADD_REACTIONS`, logged once as `discord.reaction.failed`. The tool loop
marks the turn through the turn context, **so a transport with no reaction surface is simply inert**.

## The life of a reaction

**The eyes and the hammer describe work happening**, so both are removed on the way out. **The warning
and the no entry describe how the turn ended**, so they stay, and only a mark actually applied costs a
removal call. The marks read as a state machine rather than a log: during a turn they say what is
happening, after one they say how it ended, and **a message still showing the eyes with no answer says
something went wrong without saying so**. Removal is scoped to the bot's own identity, so a member's
reaction is never touched.

**The clear is not deferred.** It runs where a turn produced an outcome the member can see: the answer,
the failure notice, or the boundary response. A turn that dies without reaching one of those keeps its
eyes, which is the point. **A deferred cleanup would fire on that death too, including during a panic
unwind, and delete the evidence in the one case it was built for.**

**Marked before the notice, never instead of it.** A denial and a queue timeout both notify at most once
per window, so a second member inside that window gets no words. The mark is applied before the throttle
is consulted, so the outcome is still visible to whoever was throttled; marking after it would leave that
member with a message carrying nothing at all, **the one state that reads as never processed**. A queue
timeout never reaches the accepted mark either, because that lands as the turn starts and this turn never
started.

**Each mark is applied once.** A turn holds one applied set, so a repeat is dropped before it reaches the
transport, and a turn spending fourteen tool calls asks for one hammer rather than fourteen. The set is
marked before the attempt rather than after, so a reaction the bot has no permission for is refused once
instead of once per round.
