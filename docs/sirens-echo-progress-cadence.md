# Progress cadence and visibility

How a progress line behaves once it exists. See
[progress](sirens-echo-progress.md) for the line itself.

## A posted line is held before the reply replaces it

A turn just over the threshold posts a line and answers a moment later, so the
line vanishes before it is readable and the channel churns for nothing. Once a
line is posted the reply waits until it has been up for a minimum window.

An unnarrated turn is never held, which is what keeps an ordinary reply fast. A
line already visible longer than the window is not held again, so a genuinely
long turn does not pay the delay twice. A cancelled turn stops waiting rather
than sitting on the member's answer.

The failure path holds too, reaching the line through the turn context, since a
notice replacing a just-posted line churns as much as a reply does.

## Every sink call is recorded

A discarded failure made three states indistinguishable: too short to narrate,
posted and missed, or refused in silence. Post, edit, and delete now record
their outcome as `discord.progress.posted` or `discord.progress.failed`. A
refused post is still not a turn failure, but it is visible.

## See also

* [Progress](sirens-echo-progress.md) - the line and when it appears.
* [Reactions](sirens-echo-reactions.md) - the other harness-state surface.
