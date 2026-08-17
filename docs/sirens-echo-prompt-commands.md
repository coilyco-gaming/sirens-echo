# A server prompt as a slash command

An MCP server publishes prompts. Discord publishes slash commands. The mapping
is mechanical and the constraints are not, because a prompt name is
server-supplied and satisfies none of Discord's shape by construction.

## Refuse rather than repair

A malformed command fails the **whole registration**, so one server's prompt
would cost every other command. That makes refusing a single prompt the
cheaper failure, and it is why this returns an error rather than inventing a
usable form.

| condition | outcome |
| --- | --- |
| name cleans to nothing | refused |
| no description | refused |
| more arguments than Discord allows | refused |
| description over the limit | truncated |

Truncation is the one repair, because a long description is still a true one
and refusing it would cost the set for a cosmetic breach.

An empty description is refused rather than filled from the name. A command
whose description restates its own name tells a member nothing, and a
registration full of those is worse than a shorter list.

## What the name mapping does

Lower cases, replaces spaces and separators with hyphens, drops anything
Discord refuses, and trims the result. A name that cleans to nothing has no
honest command form, and inventing one would register a command nobody
declared.

## The part that is not mechanical

A prompt is **user-selected instruction reaching the model through a structured
channel**. That is the same class as an uploaded file: data the turn may read,
never instructions it obeys. No filter separates a prompt that describes an
instruction from one that issues it, so the bound is posture rather than
detection, exactly as it is for [attachments](sirens-echo-attachments.md).

## Deliberate groundwork for sirens-echo#127

`CommandFromPrompt` has no production caller and that is the intended state,
not an abandonment. It is the mapping half of sirens-echo#127, named as such in
the commit that introduced it, and #127 is open with Kai's approval recorded.

**The access-policy gap is not what is holding it.** #127 opened on the concern
that a slash command is a summon path the policy does not model, and that gap
closed: `docs/access-policy.reference.yaml` names all six summon paths, slash
commands included, and `onInteraction` puts an interaction through the same
`access.Evaluate` a mention takes.

What is left is smaller and more concrete:

* **Registration is a live API call**, and its failure mode is a malformed set
  in a real guild. Rendering is pure and testable, so the split is where the
  verifiable work stops.
* **The promotable-prompt allowlist**, decided on #127. A prompt is not a
  command until this repository says it is, and its argument schema is declared
  here rather than taken from the publishing server. Nothing declares that list
  yet.
* `SIRENS_ECHO_DISCORD_COMMANDS` **defaults false** and neither lane sets it,
  so the surface that does exist is dark rather than reachable.

Boot-time prompt discovery was the fourth reason and is no longer one:
sirens-echo#163 closed.

Delete this file only alongside #127. A caller count is not the reason to.

## See also

* [Access](sirens-echo-access.md) - a slash command is a summon path and passes
  the same gate a mention does.
