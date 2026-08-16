# Prompt snapshots

The rendered prompt is checked in, so a change to what the model is told shows
up as a diff rather than as a behaviour someone notices later. What the prompt
is assembled from is [the rendered prompt](sirens-echo-prompt.md).

```sh
just prompt-dump    # rewrite the snapshots
just prompt-check   # fail when a snapshot is stale
```

`prompt-check` also runs as a pre-commit hook over `agent/`, `.agents/skills/`,
`prompt.go`, `skillpack.go`, and the dumper, so a prompt change cannot land
without its rendered diff.

## What a snapshot holds

Each snapshot carries the definition path, identity, response style, policy
roots, and system-prompt byte count, then the system prompt, the turn context,
and the user message from a fixed sample. The sample keeps those sections
deterministic, so a diff there means the framing changed.

The turn is three messages. System prompt, then the conversation around the
request as its own user turn, then the member's message alone, so a downstream
reader of "what did the user ask" gets the question. History stays flattened
and labelled inside the context message, because a Discord channel is
multi-party and the assistant and user roles cannot say which human spoke.

The files are byte-exact, so `trailing-whitespace` and `end-of-file-fixer` skip
`agent/rendered/`. Editing one by hand is pointless: the hook regenerates from
source and fails on the difference.

## Reviewing a change

A diff under `agent/rendered/` is the honest answer to "what did this change
tell the model". Read it before approving a change to any policy root, since a
one-line `SKILL.md` edit can move hundreds of bytes of context. The header byte
count is the cheapest signal of an accidental blow-up.

The byte count is worth reading even when the diff looks small. A skill root
that gained a section costs every turn that loads it, and the snapshot header
is the only place that cost is stated as a number.

## See also

- [the rendered prompt](sirens-echo-prompt.md) - sources and shared framing.
- [response profiles](response-profiles.md) - what selects a style.
