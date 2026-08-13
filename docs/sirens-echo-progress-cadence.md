# Progress cadence and visibility

How a progress line behaves once it exists. See
[progress](sirens-echo-progress.md) for the line itself.

## The line starts a grid, and everything later lands on it

A turn just over the threshold posts a line and answers a moment later, so the
line vanishes before it is readable and the channel churns for nothing. The
line therefore starts a beat. It posts at five seconds, and every message
after it releases on a ten second grid measured from that post.

Only the five is written down. The beat is twice the wait, and the long-reply
window is the wait plus two beats, which is the twenty five seconds the table
below reaches. One number moves all three, and a test pins both the derivation
and today's values so a derivation that quietly stopped deriving cannot pass.

| moment | what happens |
| --- | --- |
| 5s | the line posts, and anything ready now goes now |
| 5.1s | ready, and held |
| 15s | it posts |
| 15.1s | ready, and held |
| 25s | it posts |

The grid does not stop. A turn still running at the tenth beat waits for the
eleventh, so the hold is at most one window and averages half of one however
long the turn runs. Landing exactly on a beat is on time, since rounding a
punctual reply up to a whole extra window would be the cadence working against
the member.

An unnarrated turn is never held, which is what keeps an ordinary reply fast: a
reply before five seconds posts no line, so there is no grid to wait for. A
cancelled turn stops waiting rather than sitting on the member's answer.

The failure path holds too, reaching the line through the turn context, since a
notice replacing a just-posted line churns as much as a reply does. That means
a dead turn can show a stale line for up to one window before the notice
replaces it. A notice that jumped the grid would make failure the one thing
that answers instantly.

Edits ride the same beat. An edit is bounded by the same ten seconds since the
last one, and the post counts as the first, so the first edit lands with the
first beat rather than on a cadence of its own.

## Every sink call is recorded

A discarded failure made three states indistinguishable: too short to narrate,
posted and missed, or refused in silence. Post, edit, and delete now record
their outcome as `discord.progress.posted` or `discord.progress.failed`. A
refused post is still not a turn failure, but it is visible.

## See also

* [Progress](sirens-echo-progress.md) - the line and when it appears.
* [Reactions](sirens-echo-reactions.md) - the other harness-state surface.
