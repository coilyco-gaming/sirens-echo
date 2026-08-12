# Features

## Discord response service

- Deploy-selected verified role bundle for Deep, none for Echo
- Neutral Sirens Echo and social CoilyCo profiles with independent policy roots,
  a Kai-only trust boundary, and build-time checks
- Definition-selected history budget, skill roots, MCP roster, issue tracker
- Mention-or-reply invocation with channel, thread, guild, author, and
  duplicate gates
- Git-tracked access policy stacking guild, channel, user, and role grants with
  a deny list, per-guild rate overrides, and CI validation
- Per-user, per-context, and global admission control with a bounded queue,
  one cooldown notice per window, and bounded lookups
- Serialized turns with bounded Discord-history continuity
- Agent Proxy loop for MCP schemas, tool calls, results, and continuation
- Impersonal response contract rejecting greetings, emojis, banter, sign-offs,
  and open-ended offers
- Public Eco MCP for current game information
- Private Forgejo MCP fixed to this repository with issue create, close,
  comment, label changes, and bounded reads
- No Forgejo token in the Echo pod, one guarded handler serving the model's
  issue tools
- Plain-text replies with one style-aware repair, bounds, grounding checks, and
  neutral-style validation where selected
- Soft-reference replies with every Discord mention disabled
- Repo-local Sirens knowledge with no automatic memory or autonomous edits
- Guarded Discord context skill with SSM fast paths and bounded MCP reads
- Model-filed ordinary Forgejo issues through the guarded tool, with
  exact-title reuse and no labels
- Transport-aware OpenTelemetry ingress and joined turn traces end to end
- Trace-correlated metadata logs with byte counts and no member or model text
- Turn, latency, model-call, tool-call, admission, and failure metrics, plus 23
  bounded SigNoZ exception groups with stage and outcome tags
- Metrics-only liveness and non-generating route readiness, bounded
- Private HTTP entrypoint over the same turn path, served as JSON and as an MCP
  tool, with W3C tracing and Discord's admission policy
- Transport-neutral CoilyCo profile with no assumed domain, MCP, automatic
  issue tracking, or default write surface
- Offline harness, MCP fixtures, a non-mutating gate, and a graded board

See [the service](sirens-echo.md), [profiles](response-profiles.md),
[MCP tools](sirens-echo-tools.md), [access](sirens-echo-access.md),
[admission](sirens-echo-admission.md), [contexts](sirens-echo-contexts.md),
[HTTP](sirens-echo-http.md), and [rollout](sirens-echo-rollout.md).

## Configuration and deployment

- Deploy-selected definition, ingress switch, instance, and Agent Proxy route
- Independent community and general-purpose definitions in one immutable image
- Deployment-owned Forgejo MCP URL in Echo, repository-scoped token only in the
  MCP pod
- ExternalSecret injection with no pod AWS permission
- Existing SigNoZ collector for OTLP/HTTP traces and metrics
- Singleton k3s Echo Deployment
- Full-source-SHA Echo images published to Forgejo OCI

See [deploy.md](deploy.md).

## Development gates

- Ward verbs for build, policy verification, prompt snapshots, format, vet,
  test, tidy, run, per-profile evaluation, and full pre-commit
- Forgejo CI builds, checks policy, vets, tests, and runs pre-commit
- Structure, skills, links, modules, comments, secrets, and prompt validated

## Deliberate exclusions

- Echo has no moderation, account, role, announcement, Forgejo edit, delete,
  reaction, cross-repository, or ambient-channel surface, and sends no
  unsolicited direct message
- Echo owns no web or mobile UI

## See also

- [README.md](../README.md) - human-facing introduction
- [AGENTS.md](../AGENTS.md) - agent-facing rules
- [.ward/ward.yaml](../.ward/ward.yaml) - allowlisted commands

Cross-reference convention from [features-release-tooling.md](features-release-tooling.md), tracked by [coilysiren/agentic-os#59](https://github.com/coilyco-flight-deck/agentic-os/issues/59).
