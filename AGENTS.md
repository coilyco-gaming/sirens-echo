# Agent instructions

Workspace conventions load globally through `agentic-os-kai/AGENTS.md`. This file holds repo-local specifics.

## Scope

This repository builds the Agent Proxy-backed `sirens-echo` runtime for neutral Sirens Echo Discord and general-purpose CoilyCo HTTP profiles.
Their policy, MCP, evaluation, telemetry, and boundaries live here.

## Project shape

Community code lives in `cmd/sirens-echo`, `cmd/sirens-echo-eval`, and
`internal/community`. Definitions and evaluation cases live in `agent/`.
Local skills live under `.agents/skills/`. `sirens-echo-community` and
`sirens-echo-knowledge` are Echo's compiled local policy and knowledge.
`coilyco-general` is the domain-neutral HTTP profile.
`ops-social-discord` guides guarded read-only investigations and is not part of
the runtime skill roots. The Dockerfile packages Echo on the full AOS release
image.

## Repo boundaries

This service owns its model policies, integration, and local skill roots. Its
deployment-owned Agent Proxy alias selects inference tuning only and does not
add model instructions. Agent Proxy owns inference transport. MCP servers own
their tool behavior.

Each definition explicitly selects its channel, MCP roster, and optional issue
tracker. Sirens Echo may call its Eco and repository-fixed Forgejo MCPs and
reply in `#bots`. The CoilyCo profile selects a Steam reader and that same
Forgejo MCP, names no channel, and has no automatic issue tracker.

## Commands

Route every dev verb through Ward: `ward exec build`, `policy-check`, `vet`,
`test`, `tidy`, and `run-echo`. Do not invoke bare `go` or `uv`.

## Validation

Run `ward exec vet` and `ward exec test` before committing. The full
pre-commit gate must pass. Never use `--no-verify`.

### Live evaluation cadence

`ward exec eval-deep` runs 5 times. `ward exec eval-echo` runs once.

`ward exec board-deep` is the human-graded board and gates nothing. It repeats
each case 5 times inside one run, so it is invoked once and its stdout is the
evidence. Never wire it into CI and never derive a pass or fail from its
`structural` field. See [the Deep board](docs/sirens-echo-board.md).

The split is load on `kai-tower-3026`, not confidence. Echo's route
`sirens-echo/default` resolves to `ornith:35b` on ollama and the AOSH router
puts `default_server` on that tower, so every Echo case pins a 35B model on the
daily driver and a run takes minutes. Deep's route `sirens-echo/deepseek`
carries `direct: null` and resolves upstream to `deepseek-v4-flash`, so it never
touches the tower and repeats are cheap.

Both need `AGENT_PROXY_URL`, `AGENT_PROXY_MODEL` naming the profile's route, and
`OTEL_EXPORTER_OTLP_ENDPOINT` pointed at the deployment's receiver. Supply
`SIRENS_ECHO_MCP_ROSTER` when a case requires a tool; without one the roster is
empty and a tool case fails for a reason that is not the agent's.

Unit tests never leave the machine, so the cadence does not apply to them.

## Safety

Required secrets live in SSM. Echo's ExternalSecret maps the Discord token,
selected model, and `#bots` identifier into the pod. The private Forgejo MCP
maps its repository-scoped token only into its pod. Echo receives only the URL.

Every community event passes channel, summon, author, duplicate, response,
grounding, and mention checks. Accepted `#bots` and private HTTP turns retain
trace-correlated metadata and byte counts without member, prompt, model, tool,
or reply bodies. Rejected events and DMs never enter the turn logger. Forgejo
issues contain sanitized summaries and no labels.

For live diagnosis, an authorized ops or director session can correlate
bounded history from the deploy-owned read-only Discord MCP with Echo's
telemetry. Follow [the observability guide](docs/sirens-echo-observability.md).

## Cross-repo contracts

Infrastructure owns k3s. Agent Proxy owns inference. This repository owns model
policy. Each MCP server owns its tools. Deploy owns rollout.

## Release

Main pushes publish the Echo image at a full source SHA through the
trusted deploy lane. The workload holds no cluster credential. Deploy owns rollout.

## Agent rules

Commit directly to main and push after each commit. No PRs unless asked.
Use neutral service nouns for Echo. She/her remains Kai's pronoun. No em dashes
or semicolons in prose.

## Checkout residency

This repo is not in Agent Compose's `repository-plan.yaml`, so it has no
resident checkout under `~/projects/<owner>/`. That is intentional. Work it
from a task-scoped temporary clone, and remove that clone once the work lands.

A temporary root can be purged at any time, so commit and push before pausing,
switching tasks, or ending a session. The remote is the only durable artifact.

## See also

- [README.md](README.md) - human-facing intro.
- [docs/FEATURES.md](docs/FEATURES.md) - shipped inventory.
- [.ward/ward.yaml](.ward/ward.yaml) - allowlisted commands.
Cross-reference convention from [features-release-tooling.md](docs/features-release-tooling.md), tracked by [coilysiren/agentic-os#59](https://github.com/coilyco-flight-deck/agentic-os/issues/59).
