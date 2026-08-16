# Who the agents belong to

Sirens Discord is the community. Coilyco Gaming is a separate organization
holding a staffing and product contract with it, the Robotics Division is the
part of Coilyco Gaming that contract is with, and Echo and Deep are that
division. Sourced from Kai on sirens-echo#806.

## Why it is a knowledge source

Every surface naming both organizations had to improvise the connection,
because nothing stated it. Two agents improvising the same relationship
separately is the drift this prevents: a member who asks Echo and then Deep
should get one account, not two that happen to agree today.

So it lives in `.agents/skills/coilyco-org`, which both profiles compose, and
nowhere else. It is the only local skill root the two share, which is what
makes "both agents read the same text" a property a test can hold.

## What the prompt still carries

One line, naming the division. The relationship, the boundaries between the
organizations, and the answers to the provenance questions are all in the
knowledge source rather than in a Go string. sirens-echo#806 asked for that
directly: an engineer should not be writing an org description inline.

Before this, the shipped line said `Coilyco Gaming Intelligence Team`, which
named a third thing neither the issue nor the Forgejo account uses. Kai chose
`Robotics Division`.

## The separation is the load-bearing part

An agent is never Sirens Discord staff, and Sirens Discord staff are never
Coilyco Gaming employees. Both halves belong in an answer about who an agent
works for, because either alone is a different and wrong claim, and #230's
staff description depends on the same separation holding.

## Bounds

The contract exists and its terms are not something to disclose. Nothing in the
source names a person, an account, or an internal system, so who specifically
is behind either organization is unknown in the ordinary way.

## Copy ownership

The wording is house voice and belongs to Content Creator. What ships now is
drafted from the sourced fact on the issue at Kai's direction, and editing it
is a one-file change with no code behind it.

## See also

- [the prompt](sirens-echo-prompt.md) - where the shared framing renders.
- [the prompt budget](sirens-echo-prompt-budget.md) - what this cost per turn.
