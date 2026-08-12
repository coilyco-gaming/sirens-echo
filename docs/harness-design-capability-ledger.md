# Harness design capability ledger

Harness features with the principle behind each one, and what Sirens Echo
currently implements against it. Status is derived from code, tests, and shipped
documentation, never from a plan or an issue description.

Status vocabulary: **Complete**, **Partial**, **Not implemented**, and
**Deliberately excluded** for a boundary that is a decision rather than a gap.
Forgejo issues remain the temporary work units for closing gaps.

The ledger runs across three pages: authority and identity here,
[delivery and tools](harness-design-capability-ledger-delivery.md), then
[response and safety](harness-design-capability-ledger-response.md).

- **Natural-language judgment** - Models interpret intent without harness keyword maps. ([ref][model-spec])
  - **Implementation** - Complete. No keyword routing exists. The model reads the request and picks tools from their schemas.
- **Harness authority** - Harnesses own permissions, execution, and budgets. ([ref][mcp-architecture])
  - **Implementation** - Complete. `AccessPolicy`, `rateLimiter`, the completion budget, and the MCP phase bounds all live here, and no provider can widen them.
- **Capability ownership** - Providers own tool meaning and behavior. ([ref][mcp-architecture])
  - **Implementation** - Complete. `validateMCPServer` checks an entry's shape only, never which server is acceptable, and tool descriptions pass through untouched.
- **Role composition** - Harnesses assemble explicit role instructions. ([ref][model-spec])
  - **Implementation** - Complete. `BuildSystemPrompt` assembles identity, policy, and local roots, and a composed profile fails startup without its materialized bundle.
- **Identity binding** - Identity, attribution, and allowed actions stay aligned. ([ref][mcp-lifecycle])
  - **Implementation** - Complete. `identity` and `audit_role` come from the tracked definition and travel as `X-Ward-Role`, `X-Ward-Harness`, and request metadata.
- **Prompt assembly** - Instruction sources have a defined order and conflict policy. ([ref][model-spec])
  - **Implementation** - Complete. Sections join in a fixed order, an empty one drops out, and `agent/rendered/*.prompt.txt` is gated on drift.
- **Trust labeling** - Policy, input, context, and tool output keep distinct trust levels. ([ref][owasp-prompt])
  - **Implementation** - Complete. Policy is the system message, conversation is its own user turn, tool output is `role: tool`, and server resources are framed as data.
- **Policy isolation** - Untrusted content cannot change harness policy. ([ref][owasp-prompt])
  - **Implementation** - Partial. The prompt states the boundary and `ValidateSystemPrompt` proves the policy rendered, but nothing mechanically stops a persuasive message from being obeyed.

[model-spec]: https://model-spec.openai.com/
[mcp-architecture]: https://modelcontextprotocol.io/specification/2025-06-18/architecture
[mcp-lifecycle]: https://modelcontextprotocol.io/specification/2025-06-18/basic/lifecycle
[owasp-prompt]: https://cheatsheetseries.owasp.org/cheatsheets/LLM_Prompt_Injection_Prevention_Cheat_Sheet.html
