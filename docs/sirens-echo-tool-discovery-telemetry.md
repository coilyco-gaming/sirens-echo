# Tool discovery telemetry

How to count what tool discovery actually did, as opposed to how often it was
asked. See [the roster](sirens-echo-mcp-roster.md) for the caching itself.

## The span is the lookup, not the round trip

`mcp.tools.list` wraps the call site, so it is emitted once per turn whether or
not anything reached the network. Before the roster cache those were the same
number. They are not now.

Counting the spans after the cache landed reported one listing per turn, which
is the signature of the defect the cache was built to fix, while the service was
actually making about five round trips in three hours. Eleven of sixteen spans
in that window never left the process.

## So the span says which it was

| Attribute | Meaning |
| --- | --- |
| `mcp.tools.listed` | how many rostered servers actually listed |
| `mcp.tools.cached` | true when none did |

Count round trips with the attribute. Do not count spans, and do not read the
duration.

## Why not the duration

It works and it is not a contract. A cache hit was around a tenth of a
millisecond and a round trip around sixty, so a 500x gap was standing in for a
field that did not exist. That reading is correct until a server answers in
under a millisecond or the process gets slower, and nothing would announce the
change. See sirens-echo#520.

The service already prefers recording state where it happens rather than
reconstructing it later. A tool outcome is classified where the call completes,
because waiting until the reply is assembled loses what matters. This is the
same argument one layer out.

## What the steady state looks like

A transport that can send `tools/list_changed` never expires, so it lists on a
first listing and on a change. A transport that cannot expires at the roster
refresh interval. Neither is per turn.

A count that is still one per turn therefore means the cache is being missed
rather than absent, and the likeliest cause is connections not surviving between
turns. That is a different defect from a missing cache and wants its own issue.

## See also

- [the roster](sirens-echo-mcp-roster.md) - refresh, backoff, and staleness.
- [tool call disclosure](sirens-echo-tool-disclosure.md) - the member-facing
  receipt, as opposed to this operator-facing one.
