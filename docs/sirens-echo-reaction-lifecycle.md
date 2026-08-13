# The life of a reaction

What [reactions](sirens-echo-reactions.md) mean is one question. When they are
applied and when they go away is this one.

## In flight, or an outcome

The eyes and the hammer describe work happening, so they stop being true when
the turn ends and a channel fills with marks that mean nothing. Both are
removed on the way out. The warning and the no entry describe how the turn
ended, so they stay. Only a mark actually applied costs a removal call. See
sirens-echo#475.

So the marks read as a state machine rather than a log. During a turn they say
what is happening, after one they say how it ended, and a message still showing
the eyes with no answer says something went wrong without saying so.

Removal is scoped to the bot's own identity, so a member's reaction on the same
message is never touched.

## The clear is not deferred

It runs where a turn produced an outcome the member can see: the answer, the
failure notice, or the boundary response.

That is what keeps the accepted mark's purpose intact. A turn that dies without
reaching one of those never clears, so it keeps its eyes and never gets an
answer, which is the signal that mark exists for. A deferred cleanup would fire
on that death too, including during a panic unwind, and delete the evidence in
the one case it was built for.

## Marked before the notice, never instead of it

A denial and a queue timeout both notify at most once per window, so a second
member inside that window gets no words. The mark is applied before the
throttle is consulted, so the outcome is still visible to whoever was
throttled. Marking after it would leave that member with a message carrying
nothing at all, which is the one state that reads as never processed. See
sirens-echo#476.

A queue timeout never reaches the accepted mark either, because that lands as
the turn starts and this turn never started. So that message was not missing a
final mark. It was carrying nothing from the beginning.

## Each mark is applied once

A turn holds one applied set, so a repeat is dropped before it reaches the
transport. The tool mark is what makes this matter: it is applied in the tool
round, and a turn spending fourteen tool calls asked for the same hammer
fourteen times. Discord dedupes the visible reaction, so the member never saw
the difference and the service paid a request for each one.

The set is marked before the attempt rather than after it, so a reaction the bot
has no permission for is refused once instead of once per round.

That same set is what the clear reads, so a removal is only ever attempted for a
mark this turn actually applied.

## See also

* [Reactions](sirens-echo-reactions.md) - what each mark means.
* [Admission](sirens-echo-admission.md) - what a refusal means.
