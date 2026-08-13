# MCP call telemetry

What one tool call did, as opposed to how its server was discovered. See [tool
discovery telemetry](sirens-echo-tool-discovery-telemetry.md) for the listing
side.

## What one call returned

`mcp.tool.call` carries `mcp.tool.outcome` and `mcp.tool.result_bytes`, so a
reader holding one trace can tell a call that returned rows from one that
returned none. The outcome is the same three-state `ToolOutcome` the disclosure
footer renders to members, rather than a fourth vocabulary.

Before that, the outcome reached a metric, which aggregates and cannot be joined
to a turn, and a log line. Neither is reachable from the trace in front of you,
so an investigation into a reply asserting absence stopped one step short of
what the tool actually returned. See sirens-echo#570.

## What the bound did to it

`mcp.tool.limit_bytes` and `mcp.tool.truncated` are on the same span, on every
call rather than only the truncated ones. Absence of an attribute is not
something a reader should have to interpret, and a filter on `truncated=false`
answers how often the cap does not bind.

The bound is applied before the span ends so it can be an attribute at all.
Before that it reached only `mcp.tool.result.bounded`, and a log line cannot be
joined to the trace in front of you. See sirens-echo#640.

`limit_bytes` is the cap, not the bytes delivered. Those differ by the
truncation and spill notices appended afterwards, which is what made the cap
look like it had moved. See sirens-echo#635.

## A session is not a request

The MCP transport's client spans are named `mcp.session` plus the verb rather
than `HTTP POST`. A connection that exists to stay open has a lifetime, not a
latency, and percentiles group on the span name, so two long sessions set a
whole service's `HTTP POST` p99. Only that transport is renamed. See
sirens-echo#560.

Expect those sessions to get longer now they are no longer cut at the old
whole-request timeout. That is sirens-echo#160 working rather than a regression,
and it is the reason they needed their own name first.

## See also

- [tool discovery telemetry](sirens-echo-tool-discovery-telemetry.md) - the
  listing side, and how to count round trips.
- [tool call disclosure](sirens-echo-tool-disclosure.md) - the member-facing
  receipt, as opposed to these operator-facing ones.
