# Role and voice are separate axes

The composed bundle says what a lane is accountable for. `response_style` says
how a reply reads. Echo composes `ops` and answers neutrally, which is only
strange if those are the same axis.

## They used to be the same axis by accident

The only composing lane was also the only expressive one, so the validator
encoded the coincidence: a composing profile had to contain `## Personality
meld` and a neutral one had to not. Echo's combination was not merely unusual
under that pair of rules, it was unsatisfiable.

`composedVoicePolicy` in `prompt.go` is what separates them, rendered only for a
neutral composing profile. It states the precedence outright: the bundle supplies
doctrine and judgment, the response rules win on voice, and the seat name and
pronouns the identity card carries are never spoken.

`ValidateNeutralSystemPrompt` requires that clause exactly where it stopped
requiring the meld's absence, so the property is still checked — by presence
rather than by absence, which is the stronger of the two anyway.

## Why `ops` suits a community lane

`role-ops` is the operator charter, not the infrastructure one. The
infrastructure is a *binding*: the `use-repository infrastructure / deploy /
hardware` lines in `coilyco-bridge/agentic-os-kai`'s role graph. This repository
admits no bindings, only what its request declares, so Echo inherits the charter
and none of the estate.

Read the charter with its nouns removed and it is what
`.agents/skills/sirens-echo-community` already says longhand. Evidence defines
the estate, so a URL written from memory is a fabrication. A signal is not proof,
so no lookup is claimed without a tool result this turn. A work record stays
factual, which is what a sanitized knowledge-gap draft is. Prose grants no
executable authority, which is the MCP boundary Echo states when asked for a
write. Absent authority means preserve, gather evidence, and hand over, which is
filing an issue instead of fixing.

Echo's estate is the Sirens Discord as a running system rather than a cluster.
The role is not a second voice arriving; it is the doctrine behind the voice Echo
already had.

## What the graph and the runtime say

`ops` has no entry in `agent/compose/roles.kdl`, which is deliberate rather than
pending: the skills a community lane would reach for are voice skills Echo's
neutrality rejects, and bare `ops` is the doctrine it wanted and nothing else.

`SIRENS_ECHO_ROLE` lost its `creator` default in the same change. That default
was safe while one lane composed; now a forgotten variable would answer Echo's
community in Deep's persona, so an unnamed role fails startup the way a missing
bundle does.

## What the role brings that Echo does not want

The meld — `ops` is grounded, protective, and reflective — plus a seat name and
pronouns. Protective in particular is an emotional stance, which Echo's policy
declines outright.

That is what the precedence clause is for, and it is not the only layer.
`ValidateNeutralStyle` rejects, on every reply, the first-person voice any seat
name or stance would have to travel through to reach a member. See
[identity](sirens-echo-identity.md) and [composing](sirens-echo-compose.md).
