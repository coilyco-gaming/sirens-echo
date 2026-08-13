# Features

## Discord response service

The response surface has its own inventory, which is the part that grows
fastest. See [response service features](features-response-service.md).

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
  test, tidy, run, per-profile evaluation, failure-rate measurement, and full
  pre-commit
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
- [.ward/ward.yaml](../.ward/ward.yaml) - allowlisted commands

Cross-reference convention from [features-release-tooling.md](features-release-tooling.md), tracked by [coilysiren/agentic-os#59](https://github.com/coilyco-flight-deck/agentic-os/issues/59).
