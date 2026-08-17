# Harness design capability ledger

Harness features with the principle behind each one, and what Sirens Echo implements against it.
**Status is derived from code, tests, and shipped documentation, never from a plan or an issue
description.** The vocabulary is **Complete**, **Partial**, **Not implemented**, and **Deliberately
excluded** for a boundary that is a decision rather than a gap.

## Authority and identity

- **Natural-language judgment** - models interpret intent without harness keyword maps. **Complete.** No
  keyword routing exists; the model reads the request and picks tools from their schemas.
- **Harness authority** - harnesses own permissions, execution, and budgets. **Complete.**
  `AccessPolicy`, `rateLimiter`, the completion budget, and the MCP phase bounds all live here, **and no
  provider can widen them**.
- **Capability ownership** - providers own tool meaning and behavior. **Complete.** `validateMCPServer`
  checks an entry's shape only, **never which server is acceptable**, and tool descriptions pass through
  untouched.
- **Role composition** - harnesses assemble explicit role instructions. **Complete.**
  `BuildSystemPrompt` assembles identity, policy, and local roots, and **a composed profile fails
  startup without its materialized bundle**.
- **Identity binding** - identity, attribution, and allowed actions stay aligned. **Complete.**
  `identity` and `audit_role` come from the tracked definition and travel as `X-Ward-Role`,
  `X-Ward-Harness`, and request metadata.
- **Prompt assembly** - instruction sources have a defined order and conflict policy. **Complete.**
  Sections join in a fixed order, an empty one drops out, and the rendered snapshot is gated on drift.
- **Trust labeling** - policy, input, context, and tool output keep distinct trust levels. **Complete.**
  Policy is the system message, conversation is its own user turn, tool output is `role: tool`, and
  server resources are framed as data.
- **Policy isolation** - untrusted content cannot change harness policy. **Partial.** The prompt states
  the boundary and `ValidateSystemPrompt` proves the policy rendered, **but nothing mechanically stops a
  persuasive message from being obeyed**.

## Delivery and tools

- **Delivery isolation** - chat transports adapt messages, not model behavior. **Complete.** Discord,
  HTTP, and the MCP tool all satisfy `turnIO` and share one `runTurn`, **so a transport supplies text
  and a reply sink and nothing else**.
- **Invocation gates** - delivery surfaces decide what starts a model turn. **Complete.** Guild,
  channel, member, deny, mention, thread-parent, and duplicate checks all run before a turn **and fail
  closed**.
- **Conversation continuity** - context preserves the right thread and audience. **Partial.** Each turn
  reads `max_context_messages` from its own channel and threads stay scoped, **but the service holds no
  state between turns**.
- **Progressive disclosure** - harnesses expose capabilities only when relevant. **Partial.** Every
  roster tool is offered on every turn; prompts are caller-selected and resources reach the model only
  when a server marks them for the assistant.
- **Semantic tool negotiation** - models select capabilities by meaning, not keyword maps. **Complete.**
  Names, descriptions, and input schemas reach the model unaltered. **Selection quality is a separate
  question** (#42).
- **Tool schemas** - providers define inputs, outputs, effects, and failures. **Complete.** Published
  `InputSchema` becomes the function parameters, **and a call naming an undiscovered tool is refused**.
- **Tool-call budgets** - harnesses bound executable requests and cost. **Complete.** Six tool rounds,
  8 KB per re-injected result, and one call bounded well inside the turn.
- **Tool continuation** - results return to the same turn with its limits intact. **Complete.** The
  assistant tool-call turn and each result re-enter as their own messages, reasoning content preserved,
  under the same bounds.

## Response and safety

- **Structured response contracts** - machine-consumed output follows a validated schema. **Partial by
  decision.** The HTTP and MCP surfaces answer in a fixed shape, and **the model reply is plain text
  deliberately rather than by omission** (#102).
- **Grounding validation** - tool-based claims must follow from observed output. **Complete.**
  `ValidateGrounding` rejects invented channel references and any action claim no completed tool call
  supports.
- **Bounded repair** - repair and retry loops stay small, then fail closed. **Complete.** One response
  repair and two budget raises on distinct triggers, **then the turn fails with an error naming the
  bound it hit**.
- **Action confirmation** - models claim actions only after harness confirmation. **Complete.** The
  prompt forbids an unconfirmed claim and `actionClaimSupported` enforces it per verb against the
  executed tool set.
- **Human escalation** - consequential ambiguity crosses a visible decision boundary. **Deliberately
  excluded.** Elicitation, sampling, and roots are out of scope, **so ambiguity is stated in the reply
  rather than turned into a request for input**.
- **Mention safety** - generated text cannot create unintended notifications. **Complete.** Every reply
  sends an empty `AllowedMentions.Parse` with `RepliedUser` false, **so no generated text can notify
  anyone**.
- **Secret isolation** - credentials stay outside prompts, logs, and model arguments. **Complete.**
  Secrets arrive from the environment, the MCP workload holds its own token, telemetry keeps byte counts
  without bodies, **and a notice carries a fixed phrase rather than an upstream string**.
- **Provider portability** - model and tool providers sit behind verified contracts. **Complete.**
  Inference sits behind `CompletionClient` and Agent Proxy, tools behind `ToolProvider` and the official
  SDK, **and no definition names a physical model**.
- **Harness evaluation layers** - core behavior and live delivery get separate tests. **Complete.** Unit
  tests never leave the machine, and `eval-echo` and `eval-deep` run the production prompt and
  validators against a live route **without Discord or Forgejo writes**.

## References

[Model Spec](https://model-spec.openai.com/),
[MCP architecture](https://modelcontextprotocol.io/specification/2025-06-18/architecture),
[MCP lifecycle](https://modelcontextprotocol.io/specification/2025-06-18/basic/lifecycle),
[MCP tools](https://modelcontextprotocol.io/specification/2025-06-18/server/tools),
[MCP elicitation](https://modelcontextprotocol.io/specification/2025-06-18/client/elicitation),
[OWASP prompt injection](https://cheatsheetseries.owasp.org/cheatsheets/LLM_Prompt_Injection_Prevention_Cheat_Sheet.html),
[OWASP secrets](https://cheatsheetseries.owasp.org/cheatsheets/Secrets_Management_Cheat_Sheet.html),
[JSON Schema](https://json-schema.org/understanding-json-schema/basics),
[Discord allowed mentions](https://docs.discord.com/developers/resources/message#allowed-mentions-object),
[Discord gateway](https://docs.discord.com/developers/events/gateway),
[conversation state](https://developers.openai.com/api/docs/guides/conversation-state),
[OpenAI evals](https://developers.openai.com/api/docs/guides/evals).
