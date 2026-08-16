# Features

## Discord response service

The response surface has its own inventory, which is the part that grows
fastest. See [response service features](features-response-service.md) for what
a turn does, [admission features](features-admission.md) for who may start one,
and [observability features](features-observability.md) for what the service
reports about itself.

## Configuration and deployment

- Deploy-selected definition, ingress switch, instance, and Agent Proxy route
- Independent community and general-purpose definitions in one immutable image
- Deployment-owned Forgejo MCP URL in Echo, repository-scoped token only in the
  MCP pod
- ExternalSecret injection with no pod AWS permission
- Deploy-selected job store: in-memory, a mounted directory, or Postgres
- Existing SigNoZ collector for OTLP/HTTP traces and metrics
- Singleton k3s Echo Deployment
- Full-source-SHA Echo images published to Forgejo OCI

See [deploy.md](deploy.md).

## Development gates

- just recipes for build, policy verification, prompt snapshots, format, vet,
  test, tidy, run, per-profile evaluation, failure-rate measurement, and full
  pre-commit. `.ward/ward.yaml` no longer carries commands and is catalog
  metadata only
- Every boundary this deployment holds declared once in
  [eval/boundaries.yaml](../eval/boundaries.yaml), with `just boundaries`
  printing the paired case list and `just boundaries-check` failing when a
  declaration no longer resolves against the source it names. See
  [the evaluation board](sirens-echo-eval-board.md)
- Forgejo CI builds, checks policy, vets, tests, and runs pre-commit
- Structure, skills, links, modules, comments, secrets, and prompt validated
- Entrypoint failures logged as severity-carrying JSON, never bare stderr. See
  [exit paths](sirens-echo-exit-paths.md)

## Deliberate exclusions

- Echo has no moderation, account, role, announcement, Forgejo edit, delete,
  reaction, cross-repository, or ambient-channel surface, and sends no
  unsolicited direct message
- Echo owns no web or mobile UI

## See also

- [README.md](../README.md) - human-facing introduction
- [AGENTS.md](../AGENTS.md) - agent-facing rules
- [justfile](../justfile) - development recipes
- [.ward/ward.yaml](../.ward/ward.yaml) - catalog metadata only

Cross-reference convention from [features-release-tooling.md](features-release-tooling.md), tracked by [coilysiren/agentic-os#59](https://github.com/coilyco-flight-deck/agentic-os/issues/59).
