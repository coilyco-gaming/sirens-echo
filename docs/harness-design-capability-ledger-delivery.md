# Capability ledger, delivery and tools

Page two of [the capability ledger](harness-design-capability-ledger.md), which
holds the purpose and the status vocabulary.

- **Delivery isolation** - Chat transports adapt messages, not model behavior. ([ref][mcp-architecture])
  - **Implementation** - Complete. Discord, HTTP, and the MCP tool all satisfy `turnIO` and share one `runTurn`, so a transport supplies text and a reply sink and nothing else.
- **Invocation gates** - Delivery surfaces decide what starts a model turn. ([ref][discord-gateway])
  - **Implementation** - Complete. Guild, channel, member, deny, mention, thread-parent, and duplicate checks all run before a turn and fail closed. See [access](sirens-echo-access.md).
- **Conversation continuity** - Context preserves the right thread and audience. ([ref][conversation-state])
  - **Implementation** - Partial. Each turn reads `max_context_messages` from its own channel and threads stay scoped, but the service holds no state between turns.
- **Progressive disclosure** - Harnesses expose capabilities only when relevant. ([ref][mcp-architecture])
  - **Implementation** - Partial. Every roster tool is offered on every turn. Prompts are caller-selected and resources reach the model only when a server marks them for the assistant.
- **Semantic tool negotiation** - Models select capabilities by meaning, not keyword maps. ([ref][mcp-tools])
  - **Implementation** - Complete. Names, descriptions, and input schemas reach the model unaltered and it selects from them. Selection quality is a separate question, tracked in #42.
- **Tool schemas** - Providers define inputs, outputs, effects, and failures. ([ref][mcp-tools])
  - **Implementation** - Complete. Published `InputSchema` becomes the function parameters, and a call naming an undiscovered tool is refused.
- **Tool-call budgets** - Harnesses bound executable requests and cost. ([ref][mcp-tools])
  - **Implementation** - Complete. Six tool rounds, 8 KB per re-injected result, and one call bounded well inside the turn. See [the budget](sirens-echo-budget.md).
- **Tool continuation** - Results return to the same turn with its limits intact. ([ref][mcp-tools])
  - **Implementation** - Complete. The assistant tool-call turn and each result re-enter as their own messages, reasoning content preserved, under the same bounds.

[mcp-architecture]: https://modelcontextprotocol.io/specification/2025-06-18/architecture
[mcp-tools]: https://modelcontextprotocol.io/specification/2025-06-18/server/tools
[discord-gateway]: https://docs.discord.com/developers/events/gateway
[conversation-state]: https://developers.openai.com/api/docs/guides/conversation-state
