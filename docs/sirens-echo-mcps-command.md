# /mcps

`/mcps` reports the MCP servers this deployment reaches and the tools each one
advertises. It takes no arguments and submits no job.

```text
eco (7): get_market, get_stores, get_world, ...
forgejo (12): close_issue, comment_issue, create_issue, ...
steam: did not answer this turn
```

## It reads what the process reached, not what the file declares

The roster file names servers. Whether one answered is a different fact, and the
two come apart exactly when someone needs this command. So the reply is built
from the discovered tool set, and the configured roster supplies the names, so a
server that returned nothing is reported rather than dropped.

Three states are distinguished, because collapsing them hides the interesting
one:

* `name (n): tools` — answered, with what it advertises.
* `name: no tools` — answered, advertising nothing.
* `name: did not answer this turn` — configured and unreachable.

## It carries no addresses

An MCP entry holds a URL, a transport, and an environment map. None of that is
rendered. The addresses are in-cluster and deployment-owned, and the same
reasoning that keeps them out of the boot capability log keeps them out of here:
a name is a fact about the deployment, an address is an identifier.

A test asserts the absence rather than leaving it to review.

## It answers the caller alone

The reply is ephemeral. Introspection belongs to whoever asked, and Echo's
channel carries members who did not. That also keeps the tool surface out of the
channel as a durable, searchable artifact.

The declaration carries the flag rather than the handler, so the refusal paths —
not permitted, rate limited, bad arguments — inherit it. A denied `/mcps` does
not become the public event the command was avoiding.

## It needs no job system

`runCommand` answers it above the job guard. Reporting the tool surface is not
job work, and a deployment running with jobs off still has a tool surface worth
reporting.

## Bounds

The reply is cut to fit one interaction and says so when it cut something. A
silently short list is indistinguishable from a short roster, which is the
failure this command exists to prevent.

See [structured commands](sirens-echo-commands.md) and
[the MCP roster](sirens-echo-mcp-roster.md).
