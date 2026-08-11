# MCP roster

The roster names the MCP servers a deployment gives Echo, and how to reach each
one. Deployment chooses which servers run, the way an operator chooses their own
MCP servers. Echo validates that an entry carries the fields its transport needs
and none belonging to another, and makes no judgement about which server is
acceptable.

## Transports

`transport` selects one of three, defaulting to `streamable` when omitted so an
entry written before transports were selectable keeps working.

- `streamable` - the current HTTP transport. Takes a URL.
- `sse` - the 2024-11-05 HTTP and SSE transport. Takes a URL.
- `stdio` - a child process speaking newline-delimited JSON over its pipes.

A URL transport takes exactly one of a literal `url` or a deployment-owned
`url_env` naming the variable to read, and carries no `command`, `args`, or
`env`. A `stdio` entry takes a `command`, optional `args`, and an optional `env`
list, and carries no `url` or `url_env`.

```yaml
mcp_servers:
  - name: eco
    url: https://eco-mcp:9000/mcp
  - name: forgejo
    url_env: SIRENS_ECHO_FORGEJO_MCP_URL
  - name: local
    transport: stdio
    command: /usr/bin/some-mcp
    args: ["--read-only"]
    env: ["SOME_TOKEN"]
```

## What a stdio child inherits

Nothing except the variables its `env` list names. Echo's own environment
carries the Discord token, the Agent Proxy route, and the readiness endpoint,
so handing all of it to every child would leak credentials the server never
needed. Naming a variable forwards it. Naming one that is unset forwards
nothing rather than an empty value.

The child is bound to the session's context, so it exits when the session does
rather than outliving it.

## Trust

A `stdio` entry names a command Echo executes. Whoever can write the roster can
therefore run a process inside Echo's pod. That is the intended model, matching
the fact that the same layer already chooses Echo's image, arguments, and
mounts. It does mean the roster source deserves the write protection the pod
spec gets, and that sourcing it from a channel with weaker controls grants more
than it appears to.

## Failure

Connection is per server. One that cannot connect or list contributes no tools
and the turn continues on the rest, named to the model so it reports the gap.
Only an entirely unreachable roster stops the turn. See
[runtime MCP tools](sirens-echo-tools.md).

## See also

See [configuration](sirens-echo-config.md) and
[runtime MCP tools](sirens-echo-tools.md).
