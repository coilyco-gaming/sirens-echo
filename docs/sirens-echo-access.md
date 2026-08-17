# Access policy

The access policy is the deployment-owned answer to **who may summon this service**: a YAML file
mounted from a ConfigMap and tracked in git, named by `SIRENS_ECHO_ACCESS_POLICY`. This repository owns
the schema, the parser, and the reference copy at `docs/access-policy.reference.yaml`. Deployment owns
the values, and **the runtime never reads the reference copy**.

```yaml
schema: coilyco-harness.access.v1
deny:
  users: ["<user-id>"]        # evaluated before every allow rule
direct_messages:
  allow: ["<user-id>"]        # absent or empty answers no direct message
guilds:
  - id: "<guild-id>"
    note: "partner guild"     # never matched on, but a snowflake diff is unreviewable without it
    channels: ["<channel-id>"] # or the literal `all`
    users: ["<user-id>"]       # or `all`, which then requires rate_limit.per_user
    roles: ["<role-id>"]
    rate_limit: {per_user: "2/60s"}
```

`all` is spelled out rather than punctuated so it greps and cannot be reached by a typo. Evaluation
order is `deny.users` (which beats every allow rule including a role grant), then direct messages, the
guild entry where an unlisted guild is refused outright, the member as the union of `users` and `roles`,
then the channel or its parent when the summon arrived in a thread. **The first four decide from the
Gateway payload in memory, so anything outside the policy costs nothing**, and only an unseen thread
reaches a bounded lookup.

`roles` scales in a guild whose membership the operator does not control: listing users makes every new
person a config change plus a rollout, while a role grant covers members nobody enumerated, free,
because `Message.Member.Roles` arrives on the Gateway payload. **`staff_roles` is a separate list that
grants nothing**: it marks holders for a relaxed content posture, and a guild listing only staff roles
still allows no member.

**Absent configuration never widens the surface.** A missing or unparsable file, an unknown field, a
non-snowflake ID, a duplicate guild, a guild allowing no channel or no member, and a `users: all` guild
whose `rate_limit.per_user` is absent or `off` are all startup failures. A deployment that sets no
`SIRENS_ECHO_ACCESS_POLICY` gets the equivalent synthesized from `DISCORD_CHANNEL_ID`,
`DISCORD_GUILD_IDS`, and `SIRENS_ECHO_DISCORD_DM_ENABLED`, so one runtime representation keeps the gate
single-pathed. `sirens_echo.access.checks` carries a closed-set `reason` and **no identifier reaches a
label**.

## Validating a policy offline

`sirens-echo-access-check` answers one question: would this pod accept this access policy. It reads
files and does nothing else, so a sealed CI container can run it. It prints `<path>: ok` and what the policy
admits, exits 1 with the reason on stderr for a file that fails, and exits 2 with usage when handed no
arguments. `-` reads one policy from stdin.

**Every file in deploy fails this check when passed as a path**, including the correct ones: they are
Kubernetes manifests with the policy nested under `data["access-policy.yaml"]`, and the runtime never
sees that wrapper because the ConfigMap projects the key as a file. So deploy extracts the key first
with `yq '.data."access-policy.yaml"' access-policy.yml | sirens-echo-access-check -`. **That split is
the boundary rather than a workaround**: deploy owns the manifest format, this repository owns the
schema, and deploy never reimplements a parser for a format another repository owns.

Deploy's pre-commit parses the file as YAML, which catches a syntax error and nothing else, so until
this existed **the first thing to evaluate a policy was pod boot**. `check` calls
`community.LoadAccessPolicy`, the same function the agent calls at startup, and that is the whole
design: **a second implementation would be a worse gate than none**, passing policies the pod rejects
and rejecting ones it accepts, with the divergence appearing as a rollout failure CI called green.

The bound that matters most is a guild opened to every member without a real per-user rate limit.
Strict decoding catches the quieter one, where a misspelled key like `ratelimit` fails rather than being
ignored, **which a plain YAML parse elsewhere cannot see**, because the file is valid YAML and the bound
is simply absent. A policy can also be valid and still open a guild nobody meant to open, so a passing
run lists each guild's channels, members, roles, and resolved rate tiers. **An unset tier and a disabled
one are not the same**: an absent `rate_limit.per_user` inherits the deployment tier this file cannot
see and `off` removes limiting, so they read as `deployment default` and `unlimited`.

## Per-requester authority

**What a job may do is determined by who requested it, not by which pod runs it.** "Acts as the
principal" means filtered grants over one credential: the pod keeps its own identity and the harness
narrows which job kinds a given principal may cause. Per-principal credentials would be credential
brokering, adjacent to a deferred item and not to be entered by accident, and the filtered form is
honest about what happened, since one identity performed the effect on someone's behalf and the record
says whose.

The grant table lives in the access policy document, beside admission. The coupling concern is real and
accepted, because a grant answers the same deployment-owned question about the same principals in the
document a reviewer already reads, and **splitting it would mean two documents that must agree about
the same people**. **An unlisted principal is denied**, because `Evaluate` already fails closed on an
unlisted guild and authority should not be the one gate that fails open.

```yaml
grants:
  principals:
    - {id: "<user-id>", note: the trusted principal, kinds: all}
    - {id: "<user-id>", note: a guild member, kinds: ["echo"]}
```

`kinds` takes `all` or an explicit list, the same `Allowlist` shape channels and users use. **A kind not
declared in `JobKinds` fails validation**, so a table cannot grant something that does not exist, and a
malformed table stops startup rather than silently denying everyone. A deployment that declares no
`grants` block is unchanged, which is correct before a table is adopted and wrong after the guild
widens, so **adopting one is part of opening the guild rather than a follow-up**. This adds no second
authorization checkpoint beyond admission and no human-in-the-loop approval step, items 7 and 8, which
remain out of scope, and it does not decide who may **reach** the agent.

## Reporting a refusal

**A grant this deployment does not hold is permanent.** Retrying cannot satisfy it, so every surface
reporting one has to say so. `GrantTable.Permits` returns a `GrantDenial` and `IsGrantDenial` tells it
from anything else an error path carries, and both submit surfaces call it. HTTP answers `403` with
`sirens_echo.jobs.not_permitted`; before, it fell to the `default` arm and answered `400` with
`sirens_echo.jobs.rejected`, **the same answer an unknown kind or a malformed body gets**, so a caller
could not tell a request it should fix from an authority it does not have. Discord says the caller is
not permitted to start that job kind, rather than the generic notice that reads as an invitation to try
again. `503` for a full queue remains the one job refusal that is this service's fault.

**A refused submission still creates a job record**, moved to `failed` with the outcome `not permitted`,
so a denial appears in the principal's listing, carries a reason, and can be asked about afterwards. A
denial that left no record would be the one event nobody could investigate. **`403` leaks nothing**: a
principal learns only about its own grant, which `GrantedKinds` exists to answer without a refusal, and
the reason string stays out of the response body. Contrast the not-found and not-owner pair, which
deliberately share `404` so an id cannot be probed for, protecting other principals' records. This one
would protect nothing.
