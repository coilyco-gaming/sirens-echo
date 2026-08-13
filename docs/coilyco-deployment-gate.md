# CoilyCo deployment gate

What the Sirens Deep workload holds, and what an operator proves before it goes
live. The profile it loads is described in [response
profiles](response-profiles.md).

## What the workload is

The Sirens Deep workload selects the hosted DeepSeek route and loads the
CoilyCo definition. It receives its own instance, namespace, tailnet hostname,
and non-reusable Tailscale key.

Its Discord ingress refuses every guild, channel, and account its
deployment-owned [access policy](sirens-echo-access.md) does not name. It
requires both MCP endpoints its definition selects and holds no Forgejo secret,
because that credential lives only in the MCP pod.

Separate instance and separate namespace is the point rather than an
implementation detail. The two profiles differ in what they will talk about,
and a shared workload would make that difference a configuration value instead
of a boundary.

## What an operator proves first

Before live rollout, an authorized operator verifies the immutable image and
route registries, applies the Terraform-managed Tailscale service entry and
namespace RBAC, then deploys the reviewed values.

Tailnet HTTP verification must prove the general profile answers unrelated
topics without assuming a game or community domain and without weakening truth,
privacy, or action grounding.

Both halves matter. A general profile that refuses unrelated topics has not
generalised, and one that answers them by dropping a grounding rule has traded
the wrong thing for the breadth.

The neutral Sirens Echo deployment is verified separately and remains
unchanged.

## See also

- [response profiles](response-profiles.md) - the profiles and their controls.
- [the access policy](sirens-echo-access.md) - who reaches the ingress.
