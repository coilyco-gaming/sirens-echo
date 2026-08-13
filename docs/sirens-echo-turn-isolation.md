# What can reach a turn's prompt

Every input to a turn, so a claim about context bleed can be checked against a
list rather than argued. Raised by issue 265, where context from one surface
appeared to meld with another.

## The inputs

| Source | Varies per turn | Who supplies it |
| --- | --- | --- |
| Transcript history | yes | the caller, from the channel window |
| Current message | yes | the caller |
| Tool results | yes | whatever the roster serves, mid-turn |
| Principal | no | deployment |
| Composed bundle | no | the image, or the placeholder |
| Skillpack and local policy | no | the build |

Three vary per turn, and only two of those carry member content. That is the
whole surface. Nothing else is read at assembly time: `BuildSystemPrompt` and
`BuildTurnPrompt` are functions of their arguments, with no package state, no
file reads, and no environment lookups.

## What the tests pin

`turnisolation_test.go` asserts three properties, each mutation-checked against
a planted accumulator rather than trusted:

- a second turn's prompt carries nothing from the first, including its history,
  author, principal, bundle, and policy root
- the same inputs produce the same prompt however many turns ran between them
- the turn context holds exactly the entries the caller supplied, no more

The first is the one that matters. A bleed inside this repository would arrive
as a cache or an accumulator added later for a good reason, and that is exactly
what these catch.

## What they cannot catch

**A caller that supplies the wrong window.** If the Discord side hands a turn
history from another channel, assembly is correct and the prompt is wrong.
Nothing here can see that, and a trace is the only thing that settles which of
the two happened.

**Tool results.** A tool returning content from another surface puts it in the
turn legitimately as far as assembly is concerned. The roster and its guardfile
bound that, not this.

**Anything stateful added later.** A scratchpad is the live example. Echo's is
issue 287, and the design question there is per-requester versus per-channel
keying, which is precisely a context-boundary decision rather than a storage
one. Any store that outlives a turn belongs in the table above with its
own row and its own reasoning.

## See also

- [Caller history](sirens-echo-caller-history.md) - how the window is built.
- [Prompt assembly](sirens-echo-prompt.md) - section order and sources.
