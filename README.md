# sirens-echo

![sirens-echo and sirens-deep, a discord community agent harness](assets/banner.jpg)

A discord community agent harness - home of sirens echo and sirens deep

## About

Sirens Echo is an automated community response service. Its model context is a
repository-owned neutral profile plus approved Sirens knowledge. The Discord
deployment remains Sirens Echo.

- Only a mention or reply in a configured channel, or a thread under one,
  invokes the service. Everything else is ignored, direct messages included.
- A git-tracked access policy stacks guild, channel, user, and role grants
  with a deny list. Per-user, per-guild, and global limits bound spend.
- Replies disable all mentions. Twelve earlier messages give continuity.
- A deterministic validator rejects greetings, emojis, banter, and sign-offs.
- The tool roster exposes public Eco MCP and a private, repository-fixed
  Forgejo MCP.
- Unanswered questions and corrections become sanitized Forgejo issues.
- The action surface stays repository-scoped, with no moderation or roles.

No automatic memory. Guarded investigations use
`.agents/skills/ops-social-discord/`. See [the walkthrough](docs/sirens-echo.md).

## Coilyco harness

The `sirens-deep` deployment selects `agents/deep/definition.yaml`. Its model
identity is Sirens Deep of Coilyco and its scope is general-purpose.

- It loads only the domain-neutral `coilyco-general` policy and names no
  channel, so deployment selects Discord ingress, HTTP ingress, or both.
- It starts with no MCP, repository knowledge, issue tracker, or write surface
  while retaining the shared safety and transport bounds.
- One process serves several channels across several guilds, plus opt-in direct
  messages. See [multiple Discord contexts](docs/sirens-echo-threads.md).

General-purpose means topic-neutral and extensible, not universally authorized.
Future tools or knowledge must be added explicitly to the tracked definition
with their own permission boundary.

## Configuration

Every number has one home, `internal/community/config.go`, and every one of
them takes an environment override through the same helper. There is no tier of
numbers a deployment cannot reach. The list is generated into
[the reference](docs/sirens-echo-tuning.md) from the table itself, and a test
fails if a number drifts out of the file or the reference falls behind it.

Deploy selects the tracked YAML definition, Agent Proxy route, Discord switch,
channel and guild scope, admission limits, and instance name. Reachability of
`POST /v1/turn` is decided at the network layer by the deployment. See
[tuning](docs/sirens-echo-tuning.md),
[tuning a deployment](docs/sirens-echo-tuning.md),
[response profiles](docs/sirens-echo.md),
[admission control](docs/sirens-echo-admission.md), and
[deployment](docs/sirens-echo-deploy.md).

## Development

Run commands through just:

```sh
just setup
just build
just policy-check
just test
just vet
just pre-commit-all
```

`just setup` installs the pre-commit hooks. Run it once per clone.

`just eval-echo` exercises the production prompt, Agent Proxy, and static
Eco MCP roster without sending Discord messages or creating issues. Set
`OTEL_EXPORTER_OTLP_ENDPOINT` to name the evaluation target you run against.

## Deployment

Main pushes publish a full-source-SHA Echo image to Forgejo OCI.
`coilyco-bridge/deploy` owns the k3s Deployment, secrets, rollout, and
rollback. See [the rollout checks](docs/sirens-echo-deploy.md).

## See also

See [AGENTS.md](AGENTS.md), [docs/FEATURES.md](docs/FEATURES.md), [justfile](justfile), [.ward/ward.yaml](.ward/ward.yaml),
[admission](docs/sirens-echo-admission.md), [access](docs/sirens-echo-access.md),
[HTTP](docs/sirens-echo-http.md), [health](docs/sirens-echo-observability.md),
[notices](docs/sirens-echo-delivery.md),
[identity](docs/sirens-echo-identity.md),
[role record](docs/sirens-echo-compose.md),
[jobs](docs/sirens-echo-jobs.md),
[job lifecycle](docs/sirens-echo-jobs.md),
[job telemetry](docs/sirens-echo-telemetry.md),
[commands](docs/sirens-echo-commands.md),
[execution](docs/sirens-echo-execution.md),
[guardfile knowledge](docs/sirens-echo-config.md),
[reply progress](docs/sirens-echo-progress.md),
[the identity eval](docs/sirens-echo-identity.md),
[counterparts](docs/sirens-echo-compose.md),
[attribution](docs/sirens-echo-worklog.md),
[grants](docs/sirens-echo-access.md), and
[docs/sirens-echo-deploy.md](docs/sirens-echo-deploy.md).
Cross-reference convention from [FEATURES.md](docs/FEATURES.md), tracked by [coilysiren/agentic-os#59](https://github.com/coilyco-flight-deck/agentic-os/issues/59).
