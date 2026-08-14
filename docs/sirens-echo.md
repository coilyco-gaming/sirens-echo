# Sirens Echo response service

Sirens Echo binds a deployment-selected response profile to Discord or private
HTTP ingress, Agent Proxy, MCP servers, Sirens knowledge, Forgejo, and
telemetry.

This repository owns the service and its model-facing policy. Agent
Proxy owns inference transport. MCP servers own their published tools. Deploy
owns the k3s workload.

## Request flow

1. Discord delivers a guild message, or a direct message where enabled.
2. Echo accepts only a mention or reply in a configured channel or a thread
   under one, inside an allowlisted guild when an allowlist is set.
3. Admission charges the per-user, per-context, and global buckets, and sheds
   the summon if any is exhausted or the queue is full.
4. The runtime loads up to 12 earlier messages and marks them untrusted. Inside
   a thread that history and the reply stay in the thread.
5. The harness combines the selected local policy, history, and the summon.
6. It discovers the selected definition's MCP schemas and sends them to Agent
   Proxy. Sirens Echo selects public Eco and private Forgejo.
7. It executes model-requested tools and continues with their results.
8. It enforces JSON, repairs once, validates, and may create or reuse an issue.
9. It applies the selected style contract and replies with every Discord
   mention disabled.

Turns run one at a time. A second summon reads history after the first finishes,
so it sees the prior member message and Echo reply. A summon waits up to the
queue timeout for that slot, and its request budget starts once it holds the
slot. Echo keeps only bounded history and duplicate detection, with no automatic
memory. See [admission control](sirens-echo-admission.md) and
[multiple Discord contexts](sirens-echo-contexts.md).

## Private HTTP entrypoint

The same validated turn path is reachable through `POST /v1/turn` on the
process's private listener, with its own shared-secret authentication and the
same admission policy. A deployment can disable Discord and use it as the only
ingress. See [the HTTP entrypoint](sirens-echo-http.md).

## Deployment-selected response policy

Deploy selects the neutral Sirens Echo community profile or the social CoilyCo
general-purpose profile. The profiles share no knowledge or tools. Sirens Echo
retains its approved community knowledge, Eco MCP, private Forgejo MCP, and
automatic issue workflow. CoilyCo starts with one domain-neutral policy root,
no MCP roster, and no automatic issue writes.

Both profiles load an Agent Compose role bundle: `ops` for Echo, `creator` for
Deep. Echo takes the role for its doctrine and not its voice. Build-time
verification checks both profiles. See [response
profiles](response-profiles.md).

See [MCP tool behavior](sirens-echo-tools.md), [observability](sirens-echo-observability.md), and [rollout checks](sirens-echo-rollout.md).

For runtime configuration, issue handling, and evaluation boundaries, see
[configuration and evaluation](sirens-echo-config.md).
