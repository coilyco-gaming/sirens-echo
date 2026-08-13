# Runtime MCP tools

Each source-controlled definition owns its MCP roster and optional issue tracker.

The Sirens Echo community definition contains public Eco MCP and an
environment-backed private Forgejo MCP URL. Eco needs no client credential or
tailnet identity, Echo sends no credential to the private MCP, and the separate
MCP workload holds its Forgejo token.

The CoilyCo general-purpose definition selects a Steam reader and the same
repository-fixed Forgejo MCP. It names no issue tracker, so a write happens
because the model chose a tool. General-purpose describes topic scope, not
universal mutation authority.

## Tool loop

When a definition configures MCP servers, the harness:

1. Opens every configured MCP session.
2. Lists each published tool, description, and input schema.
3. Exposes each model name as `<server>__<tool>`.
4. Sends those schemas with the Agent Proxy request.
5. Executes valid model-requested tools.
6. Renders each result as text, appends the calls and results, then continues.

An empty roster skips discovery and sends no tools. Agent Proxy can return a
response containing only tool calls when tools exist. Final content can use an
OpenAI-compatible string, text-part array, or text object. Continuation stops
after six tool rounds.

## Validation and failure

The harness rejects:

- Empty, colliding, or overlong model-facing tool names
- Tool calls missing an identifier or function name
- Calls to tools absent from the discovered roster
- Tool arguments that are not JSON objects
- A response with neither tool calls nor usable content
- More than six tool rounds

A server that fails to connect or list contributes no tools and the turn goes on
with the rest, named to the model so it reports the gap. Only an unreachable
roster stops the turn, and a name collision stays fatal. An invocation failure
ends the turn with the tool-failure notice, and an MCP error result is grounded
data the model self-corrects from. See [notices](sirens-echo-notices.md).

## Acceptance coverage

The official in-process MCP fixture proves schema discovery, a tool-call-only
first model response, complete tool result continuation, a grounded user-facing
reply, alternate content forms, and that a tool which never answers fails on the
call bound rather than on the turn's. Separate profile tests prove the CoilyCo
definition selects exactly its Steam and Forgejo surfaces, resolves both
addresses from deployment, and names no issue tracker.

The live Echo evaluation selects only static MCP URLs, then requires an
`eco__get_eco_server_status` call without sending Discord or Forgejo writes.

See [the service](sirens-echo.md), [large results](sirens-echo-tool-results.md),
[issue ownership](sirens-echo-issue-ownership.md), and
[observability](sirens-echo-observability.md).
