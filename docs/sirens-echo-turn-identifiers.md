# Turn identifiers

A member who is handed a trace id can only use it if the turn it names can be
placed. The turn span therefore carries Discord's own identifiers, and this
document records which ones, why the account id is among them, and what is
still excluded.

## What the turn span carries

On a Discord turn inside a guild:

- `discord.user.id` - the author's account id
- `discord.guild.id`
- `discord.channel.id`
- `discord.thread.id` - only when the turn happened inside a thread
- `messaging.message.id` - the member's message, not the reply

The keys match the ones the send span already used, so one query returns both
halves of an exchange rather than two shapes that have to be joined by hand.

## Thread and channel are not the same field

Discord models a thread as a channel, so a naive mapping reports the thread id
as the channel id and every thread turn disappears from a query for the channel
it hangs under. That is the query an operator actually runs, so the parent is
reported as `discord.channel.id` and the thread gets its own key.

Resolution reads cached gateway state and never calls the Discord API. A
channel that is not in the cache reports itself as a channel, which is what it
is, and contributes no thread id. Absent beats blank: an empty thread id reads
as a thread nobody can find.

## The account id is here by reversal

The code previously said, in a comment beside the span, that the requester was
deliberately not a span attribute because an account id is not operational
telemetry. That was a real position and it was overturned on purpose, in
[issue 337](https://forgejo.coilysiren.me/coilyco-gaming/sirens-echo/issues/337),
by the director.

The reasoning that changed: an account id in a private telemetry backend and a
handle in a public reply are different objects. The first answers "show me
every turn this member had trouble with" and cannot be reconstructed from a
job record when the trouble is a turn rather than a job. The second is still
refused, and separately, by the reply validators.

The indirection that stood in for it - a job id in telemetry, resolved to a
principal through the job record - still holds for jobs. It does not cover
turns, because a turn that failed before a job existed leaves no record to join
against.

## What is still excluded

Prompt, model, tool, and reply bodies stay out of telemetry. So does anything
member-visible: a display name, a nickname, a handle. Identifiers go to the
backend and never into what a member reads. A direct message contributes no
attributes at all, because a DM never enters the turn logger and this change is
not the first exception to that.

The HTTP profile contributes nothing here. It carries no Discord identity, and
its request id is already on the span.

## If it should come back out

`SpanAttributes` on the Discord turn is the single place the account id is
added, and one test asserts it is present. Removing it means deleting that
assertion in the same commit, which is deliberate: the test exists so the
removal is a decision someone records rather than a line that quietly rots.

## See also

- [notices carry the trace](sirens-echo-notice-trace.md) - where a member gets
  the id in the first place.
- [attribution](sirens-echo-attribution.md) - the job-record join.
