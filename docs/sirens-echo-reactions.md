# Reactions as harness state

A reaction is applied to the member's own message and reports what the harness
is doing with it. It is not model output, so it never passes through reply
validation, and the neutral response contract does not reach it. That contract
governs the words the model writes, and a reaction contains none.

## The four cases

| Case | Mark | Applied |
| --- | --- | --- |
| accepted at harness level | eyes | as the turn starts, before any model call |
| the turn called a tool | hammer | in the tool round |
| the turn produced no reply | warning | on the failure path |
| refused at a boundary | no entry | on the admission denial |

## In flight, or an outcome

The eyes and the hammer describe work happening. When the turn ends they
describe nothing, and a channel fills with marks that no longer mean anything.
Both are removed on the way out. The warning and the no entry describe how the
turn ended, so they stay. See sirens-echo#475.

Only a mark that was actually applied is removed, so an ordinary turn that
called no tool costs one removal rather than two.

## Where the clear happens, and why not in a deferred cleanup

The clear runs where a turn produced an outcome the member can see: the answer,
the failure notice, or the boundary response. It is not deferred.

That is what keeps the accepted mark's whole purpose intact. A turn that dies
without reaching one of those paths never clears, so the message keeps its eyes
and never gets an answer, which is exactly the signal below. A deferred cleanup
would fire on that death too and delete the evidence.

## Why the accepted mark earns its place

It lands before the first model call, so a turn that then dies silently is still
visible: the message carries a mark and never gets an answer. During a model
outage that turns "the bot has gone quiet and nobody knows why" into a signal
every member in the channel can see immediately, at no alerting cost. That is
not monitoring and it pages nobody, but it converts a silent failure into a
visible one.

## A reaction can never fail a turn

Every reaction failure is swallowed. The likeliest cause is the bot identity
lacking `ADD_REACTIONS` in the channel, which is an operator question rather
than a turn outcome, and a member losing a real answer because a reaction call
was refused would be worse than having no reactions at all. The agent logs the
failure once as `discord.reaction.failed` so the permission gap is visible
without becoming an error.

## Each mark is applied once

A turn holds one applied set, so a repeat is dropped before it reaches the
transport. The tool mark is what makes this matter: it is applied in the tool
round, and a turn spending fourteen tool calls asked for the same hammer
fourteen times. Discord dedupes the visible reaction, so the member never saw
the difference and the service paid a request for each one.

The set is marked before the attempt rather than after it, so a reaction the bot
has no permission for is refused once instead of once per round.

## How the tool round reaches it

The tool loop sits behind the completion boundary and takes no transport
argument. It marks the turn through the turn context, the same route the
progress line and attribution already use, so no new mechanism is introduced and
a transport with no reaction surface is simply inert.

## See also

* [Progress](sirens-echo-progress.md) - the other harness-state surface.
* [Notices](sirens-echo-notices.md) - harness words, as opposed to harness marks.
* [Admission](sirens-echo-admission.md) - what a refusal means.
