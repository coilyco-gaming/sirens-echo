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

## Not yet built

Registration itself. Rendering a command is pure and testable; registering it
is a live API call whose failure mode is a malformed set in a real guild, and
it interacts with when prompts are known, which is the open question on
boot-time discovery.

## See also

* [Access](sirens-echo-access.md) - a slash command is a summon path and passes
  the same gate a mention does.
