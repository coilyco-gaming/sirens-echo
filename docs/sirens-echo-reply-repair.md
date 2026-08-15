# Repairing a refused reply

A reply check refuses the whole message. One block naming a channel nobody has
discarded eleven correct blocks beside it, including an outage report that was
then found in a trace hours later rather than read in the channel. See
sirens-echo#796, trace `7ea1e319b92b0357d3e2ac71b802a66a`.

## The gap this closes

A repair loop already existed in the completion layer, and already named the
refusing check to the model so it could fix the clause rather than rephrase
blind. It covered two checks: `parse` and `response_style`.

The other five ran in `runReplyChecks` after `Complete` returned, so they had no
repair at all. A grounding refusal ended a 51-second turn outright. The
machinery was there and those checks were not wired into it.

They are now. `ProxyClient.ValidateReply` takes the harness's own check set, and
the runtime wires it to `runReplyChecks`, so all ten rules reach the loop.

## Repair is advisory, and the verdict did not move

`runReplyChecks` still runs after `Complete` returns, and it is still the only
thing that refuses. The hook inside the loop is an earlier, best-effort run:
it can turn a refusal into a rewrite, and it can never turn a refusal into a
delivery.

That is the reason the loop drops the harness checks once the repair budget is
spent, rather than failing on them. Failing there would return
`ErrResponseRepairExhausted` and report `stage=model`, which is
[true about the code and false about the world](sirens-echo-turn-stages.md).
Handing the reply back unrepaired keeps the refusal, its stage, and its
`response.check` value exactly where they already were.

So a model that will not fix the clause produces the same outcome as before
this change, byte for byte. What moved is only the case where it will.

## What a repair pass may not do

Tools are already withdrawn on a repair attempt, so the model cannot answer a
grounding refusal by going and substantiating the claim. It rewrites or it
keeps the refusal. That asymmetry is deliberate: a check refusing an unsupported
claim must not become a prompt to go find support for it.

The budget is `maxResponseRepairs`, one attempt, shared with the contract
checks. A contract refusal that spends it leaves nothing for a harness refusal
in the same turn, which resolves conservatively: the reply is handed back and
refused.

## What this does not do

The member still sees a whole reply or none of it. Partial delivery, and
preserving a refused reply where an operator can read it, were the other two
options weighed on sirens-echo#796 and neither is implemented here. Preserving
the text in particular is not free: a reply refused by `identifier_disclosure`
carries the value that check exists to keep out of logs.

## Measurement is deliberately unhooked

`cmd/sirens-echo-eval` builds a `ProxyClient` with no hook, and the evaluation
scorer runs the checks itself, collecting every failure rather than the first.
That path measures raw model behaviour, so the rate packs stay comparable
across this change instead of reporting the harness's repair as the model
improving.

## See also

- [turn stages](sirens-echo-turn-stages.md) - which check refused, and the
  closed set of names.
- [why a check refused a reply](sirens-echo-refusal-reason.md) - the sentence
  the loop feeds back.
- [grounding](sirens-echo-grounding.md) - what the four grounding rules exclude.
