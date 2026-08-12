# Capability ledger, response and safety

Page three of [the capability ledger](harness-design-capability-ledger.md),
which holds the purpose and the status vocabulary.

- **Structured response contracts** - Machine-consumed output follows a validated schema. ([ref][json-schema])
  - **Implementation** - Partial by decision. The HTTP and MCP surfaces answer in a fixed shape. The model reply is plain text deliberately rather than by omission. See #102.
- **Grounding validation** - Tool-based claims must follow from observed output. ([ref][model-spec])
  - **Implementation** - Complete. `ValidateGrounding` rejects invented channel references and any first-person action claim no completed tool call supports.
- **Bounded repair** - Repair and retry loops stay small, then fail closed. ([ref][owasp-prompt])
  - **Implementation** - Complete. One response repair and two budget raises on distinct triggers, then the turn fails with an error naming the bound it hit.
- **Action confirmation** - Models claim actions only after harness confirmation. ([ref][mcp-tools])
  - **Implementation** - Complete. The prompt forbids an unconfirmed claim and `actionClaimSupported` enforces it per verb against the executed tool set.
- **Human escalation** - Consequential ambiguity crosses a visible decision boundary. ([ref][mcp-elicitation])
  - **Implementation** - Deliberately excluded. Elicitation, sampling, and roots are out of scope, so ambiguity is stated in the reply rather than turned into a request for input.
- **Mention safety** - Generated text cannot create unintended notifications. ([ref][discord-message])
  - **Implementation** - Complete. Every reply sends an empty `AllowedMentions.Parse` with `RepliedUser` false, so no generated text can notify anyone.
- **Secret isolation** - Credentials stay outside prompts, logs, and model arguments. ([ref][owasp-secrets])
  - **Implementation** - Complete. Secrets arrive from the environment, the MCP workload holds its own token, telemetry keeps byte counts without bodies, and a notice carries a fixed phrase rather than an upstream string.
- **Provider portability** - Model and tool providers sit behind verified contracts. ([ref][mcp-architecture])
  - **Implementation** - Complete. Inference sits behind `CompletionClient` and Agent Proxy, tools behind `ToolProvider` and the official SDK, and no definition names a physical model.
- **Harness evaluation layers** - Core behavior and live delivery get separate tests. ([ref][openai-evals])
  - **Implementation** - Complete. Unit tests never leave the machine, and `eval-echo` and `eval-deep` run the production prompt and validators against a live route without Discord or Forgejo writes.

[model-spec]: https://model-spec.openai.com/
[mcp-architecture]: https://modelcontextprotocol.io/specification/2025-06-18/architecture
[mcp-tools]: https://modelcontextprotocol.io/specification/2025-06-18/server/tools
[mcp-elicitation]: https://modelcontextprotocol.io/specification/2025-06-18/client/elicitation
[owasp-prompt]: https://cheatsheetseries.owasp.org/cheatsheets/LLM_Prompt_Injection_Prevention_Cheat_Sheet.html
[owasp-secrets]: https://cheatsheetseries.owasp.org/cheatsheets/Secrets_Management_Cheat_Sheet.html
[json-schema]: https://json-schema.org/understanding-json-schema/basics
[discord-message]: https://docs.discord.com/developers/resources/message#allowed-mentions-object
[openai-evals]: https://developers.openai.com/api/docs/guides/evals
