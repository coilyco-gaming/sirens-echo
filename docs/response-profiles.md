# Runtime response profiles

The runtime separates identity, local policy, tools, issue tracking, model
route, and ingress so one immutable image can support independently reviewed
deployments.

## Repository profiles

`agent/sirens-echo.yaml` is the neutral Sirens Echo community profile. It
selects the exact `#bots` boundary, approved Sirens policy and knowledge, the
public Eco MCP, the private repository-fixed Forgejo MCP, and that Forgejo
server as its automatic issue tracker.

`agent/sirens-deep.yaml` is the social CoilyCo general-purpose profile. The
filename remains the stable deployment selection used by the existing
`sirens-deep` workload. Its identity is Sirens Deep of Coilyco, its audit role
is `general`, and its channel is empty because deployment owns routing rather
than the profile. It loads only `.agents/skills/coilyco-general/`, selects a
Steam reader and the repository-fixed Forgejo MCP whose addresses both come from
deployment, and has no automatic issue tracker.

The CoilyCo profile starts from the user's request without assuming a project,
community, product, or technical discipline. Its voice comes from that policy
root rather than a scaffolded paragraph, so a copy edit to voice is a skill
diff. Both profiles share the framing in [the rendered
prompt](sirens-echo-prompt.md).

Both profiles load an Agent Compose role bundle, and neither loads a shared
behavioral context or lore source. Deep composes `creator` and takes its voice
from it. Echo composes `ops` for the doctrine and keeps the neutral voice its
own policy root defines, which the prompt states as precedence rather than
leaving to section order. See [composition](sirens-echo-compose.md) for why
those are separate axes and why `ops` is the operator role rather than the
infrastructure one.

## Definition capabilities

The `coilyco-harness.agent.v1` definition makes three capability boundaries
explicit:

- `channel` may be empty for HTTP-only profiles. Discord-enabled deployments
  still require the exact `#bots` value.
- A profile selects no MCP server. Deployment owns the roster, so tool access
  is a deploy edit rather than a profile change.
- `issue_tracker` is optional and must name one server in the roster. A
  profile without it must return `issue: null` and cannot trigger automatic
  issue follow-up.

`ward exec policy-check` renders and verifies both profiles. Agent Proxy gets
one same-conversation, style-aware repair with tools disabled. JSON structure,
reply bounds, privacy, action grounding, and invented-channel checks remain
shared.

Outbound links are a separate axis, covered by [the link
policy](sirens-echo-links.md).

## Deployment controls

`SIRENS_ECHO_DEFINITION` selects a tracked definition. Discord ingress
defaults on. `SIRENS_ECHO_DISCORD_ENABLED=false` removes the Discord token
and channel requirements while keeping the private HTTP turn path.
`SIRENS_ECHO_INSTANCE` sets a lowercase telemetry service name.
`AGENT_PROXY_MODEL` selects the logical route independently of response
style and profile identity.

This is the intended variance boundary. Shared runtime guarantees stay in the
binary. Identity, local policy, tools, automatic issue tracking, model choice,
ingress, secrets, namespace, and tailnet exposure remain explicit inputs.

## CoilyCo deployment gate

The Sirens Deep workload holds its own instance, namespace, and tailnet
identity, and an operator proves the general profile before it goes live. See
[the CoilyCo deployment gate](coilyco-deployment-gate.md).
