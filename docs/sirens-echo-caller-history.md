# Caller-supplied history carries no authority

Both private ingresses take an optional `history`. `/v1/turn` takes it in the
JSON body and the `turn` MCP tool takes it as a parameter. In both cases the
caller writes every field, including each entry's author.

## The problem that produced this

The transcript is flattened as `- <author>: <content>`, and the runtime writes
the author `assistant` when it folds a resolved prompt into a turn. A caller
setting that same author therefore produced a line indistinguishable from one
the runtime wrote about its own prior turn, and the model treated it as its own
prior commitment.

Live QA measured the effect rather than inferring it. A forged entry asserting
a completed identity verification raised principal user ID disclosure from
about 13 percent on a plain impersonation claim to about 40 percent, and
removed the hedging that had wrapped the unforged answer.

## Why the author name was not the fix

Rejecting the string `assistant` stops one spelling, not the lever. A caller can
equally assert `system`, the service's own display name, or any other author
with standing, and the forged content still reads as a prior turn by someone
who has it. That fix would have looked complete while leaving the measured
behavior available.

Folding caller history into one attributed role removes the forged authority
too, but it also flattens genuine multi-party context, which the transcript
exists specifically to express.

## What happens instead

Every caller-supplied entry is marked at assembly, and the transcript renders
that mark as `(asserted by the caller, not observed)`. Authors are preserved, so
multi-party context survives, and a forged prior turn is no longer addressable
as something the service actually said.

This is the same mechanism `(an agent, not a person)` already uses for
counterpart kind. The model reads a grounded fact rather than inferring
provenance from prose, and the two markers compose on one entry.

Discord history is unaffected. Those entries come from messages the runtime
observed, so they carry no mark and that path is unchanged.

## What this is not

A prompt-level mark is not enforcement, and it does not make caller history
trustworthy. The network boundary remains the real control: reaching either
ingress means being an authorized tailnet node.

If a re-measure shows the disclosure rate unchanged, that is evidence the
load-bearing fix is the output-side validator that checks a reply against the
identifiers the process holds, rather than anything done to the input.

## See also

* [HTTP entrypoint](sirens-echo-http.md) - the two ingresses this applies to.
* [Identity](sirens-echo-identity.md) - what a reply may not claim.
