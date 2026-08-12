# MCP resources

A server can hand Echo reference material directly, instead of Echo asking for
it through a tool call and paying a round trip plus the tool-result bound.

The MCP specification calls resources application-driven: the host decides how
to incorporate them, and automatic context inclusion is one of the patterns it
names. That is why Echo may pull a resource in without the model asking, while
it does not do the same for prompts.

## What gets included

Only a resource whose `annotations.audience` names `assistant`. Inclusion is a
deliberate server signal rather than an Echo assumption, so a server publishing
a large catalogue does not flood the prompt, and material meant for a human
reader stays out of the model's context.

Qualifying resources are ordered by `annotations.priority`, highest first, with
the URI as a stable tiebreak. Ordering matters because the bounds below cut from
the bottom.

## Bounds

At most eight documents and 8KB of text per turn. A document that would cross
the byte budget is truncated on a rune boundary and marked. A resource whose
contents are binary, or whose read fails, is skipped rather than failing the
turn.

Discovery follows the same supervision as tools. A listing is cached and
invalidated by `notifications/resources/list_changed` where the transport can
carry one, and a server that does not publish the resources capability
contributes nothing rather than erroring.

## Framing

Included documents appear in a system message below the local policy, labelled
as data to answer from and never as instructions to follow. A resource is
content from a third party, so letting it read as instruction would give any
connected server a way to redirect the turn. The specification makes the same
point: implementations must validate prompt and resource content to prevent
injection.

## Prompts are selected, never injected

MCP prompts are user-controlled, so Echo never pulls one in on its own. An HTTP
caller selects one explicitly:

```json
{
  "author": "manual test",
  "prompt": {"server": "eco", "name": "rules", "arguments": {"topic": "trade"}}
}
```

`content` is optional when a prompt is named, because the prompt is then the
request. The server's messages keep their user and assistant roles and enter the
turn as conversation, not as system instruction. With no caller content, the
prompt's final user message becomes the request and the rest becomes history.

Required arguments are checked against `prompts/list` before the fetch, so a
missing one names itself. A failure the caller can fix is returned verbatim. A
transport failure returns a generic message, because its text could carry an
endpoint.

Discord has no selection surface yet, so prompts are HTTP-only for now.

## See also

See [the MCP roster](sirens-echo-mcp-roster.md) and
[runtime MCP tools](sirens-echo-tools.md).
