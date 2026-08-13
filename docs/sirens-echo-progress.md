# Reply progress

A long turn says what it is doing. A short one says nothing, because a progress
line for a two-second reply is noise.

## What a member sees

Nothing for the first eight seconds. After that, one line in the harness notice
format that edits in place as the turn moves:

> `reading recent messages`

> `thinking`

> `calling a tool`

> `checking the reply`

The line is removed when the reply lands. The reply, or the failure notice, is
the turn's real answer, so the narration does not outlive it.

## Why a line and not an embed

#111 asks for a rich element and names the Amplitude Slack bot as the standard.
This ships the mechanism with the notice format from #134 rather than an embed,
for one reason worth stating rather than assuming: #134 fixed a literal shape
for every harness-generated message, and an embed is a different surface. Making
progress an embed while failures and cooldowns are blockquoted code spans would
read as two bots.

The mechanism is the part that had to exist either way: threshold, one message,
in-place edits, rate limiting, and removal. Restyling it as an embed is a
contained change to `TurnProgressSink` once the format question is answered, and
that answer is Kai's.

## Bounds

**Threshold.** Nothing is posted until a turn has run long enough to be worth
narrating, so the fast path makes no Discord calls at all.

**Edit rate.** Edits are bounded, so a tool-heavy turn cannot spend its budget
talking about itself. A stage that repeats is not re-sent.

**Advisory.** A failed post or edit is dropped rather than failing the turn. A
line that arrives after the reply is deleted rather than left behind.

**Mention safety.** Empty allowed mentions, as everywhere else the harness
speaks unprompted.

## Where it does not apply

HTTP and MCP answer synchronously, so there is nothing to narrate to. Only a
Discord turn gets a progress line, and the non-Discord path is a nil progress
that every method accepts.

## Narrating a wait, not only a change

Stage transitions alone are not enough. A turn changes stage twice in its first
moments, reading history and then thinking, and then sits in one stage for as
long as the model takes. Since a line is only posted on a stage change that
happens after the threshold, the first post landed on the last transition, just
before the reply, and was deleted milliseconds later. The feature was present
and effectively invisible.

A watcher therefore ticks alongside the turn and posts the current stage once
the threshold passes, whether or not anything changed. A line already up is left
alone until a real stage change edits it, so the watcher can never produce a
column of messages.

The tool loop narrates through the turn context rather than through an argument,
because it sits behind the completion boundary and takes no progress parameter.
A context without a progress line makes that call inert, which is the ordinary
case for a transport that answers synchronously.

Job progress is a separate mechanism with the same shape, because a job's origin
outlives its turn. See [job telemetry](sirens-echo-jobs-telemetry.md).

See [notices](sirens-echo-notices.md) and [the service](sirens-echo.md).
