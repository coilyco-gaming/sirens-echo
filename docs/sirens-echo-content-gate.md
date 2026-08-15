# The content gate

The taxonomy in `agent/content-classes.yaml` declared `deny: true` on six
classes and nothing read it at runtime, so asking where the bus is got whatever
the model decided rather than a refusal. This is the piece that makes the
declaration mean something.

## The switch is the deployment

`SIRENS_ECHO_CONTENT_CLASSES` names the taxonomy. Unset loads none, runs no
classifier, and costs nothing — not a model call, not a decision, not a tag
beyond `content.classified: false`. A deployment turns the gate on, the same
way it turns the scratchpad on.

That is deliberate. A gate that refuses member requests should arrive by a
decision someone made rather than by a binary being rebuilt.

## What it costs when it is on

One extra model call per turn, which lands hardest on the cheapest turns: an
ordinary one-call reply becomes a two-call reply. A six-round tool turn barely
notices. The full costing, including the interaction with the three second
progress threshold, is on
[issue 227](https://forgejo.coilysiren.me/coilyco-gaming/sirens-echo/issues/227).

## A broken gate is not a denial

A classifier that errors, times out, or answers with a class outside the closed
set leaves the turn unclassified and the turn proceeds. The failure is logged
and the span says the turn was never classified.

This is the most important property here. The alternative — treating a
classifier failure as a refusal — turns one broken dependency into a service
that refuses everything, and it would do so while looking like policy working.

## The failure carries its kind in the slug

A failed gate emits its own span under the turn, named for how it broke. See
[gate failures](sirens-echo-content-gate-failures.md).

## The four tags

- `content.classified` - whether the gate ran at all
- `content.class` - which class was decided
- `content.approved` - whether it was allowed
- `content.sensitive` - whether the class is a sensitive one

The first earns its place by being false much of the time. A turn that was
never classified carries only that tag: reporting a class for a decision that
never happened would make the other three lies.

Sensitive is separate from approved on purpose. It changes the refusal's shape
and not its verdict, because a sensitive block names no category — naming it
tells a member what to avoid saying next time.

## Ordering

The gate runs after the prompt is assembled and before the answering call, so
a blocked turn spends no completion budget on an answer nobody will read. It
runs on the member's own message rather than on the assembled prompt, because
what was asked for is the thing being classified.

## See also

- [content classes](sirens-echo-content-classes.md) - the taxonomy itself.
- [gate failures](sirens-echo-content-gate-failures.md) - the failure spans.
- [notices](sirens-echo-notices.md) - why a block is not a notice.
