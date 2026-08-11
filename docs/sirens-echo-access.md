# Access policy

The access policy is the deployment-owned answer to who may summon this
service. It is a YAML file mounted from a ConfigMap and tracked in git, named by
`SIRENS_ECHO_ACCESS_POLICY`. This repository owns the schema, the parser, and
the reference copy at `docs/access-policy.reference.yaml`. Deployment owns the
values, and the runtime never reads the reference copy.

## Shape

```yaml
schema: coilyco-harness.access.v1
deny:
  users: ["<user-id>"]        # evaluated before every allow rule
direct_messages:
  allow: ["<user-id>"]        # absent or empty answers no direct message
guilds:
  - id: "<guild-id>"
    note: "operator's own guild"
    channels: all
    users: all
  - id: "<guild-id>"
    note: "partner guild"
    channels: ["<channel-id>"]
    users: ["<user-id>"]
    roles: ["<role-id>"]
    rate_limit:
      per_user: "2/60s"
```

`channels` and `users` take the literal `all` or an ID list. `all` is spelled
out rather than punctuated so it greps and cannot be reached by a typo, and a
misspelling is a startup failure rather than a silent widening. `note` exists
because a diff of 19-digit snowflakes is otherwise unreviewable, and the loader
never matches on it.

## Evaluation order

1. `deny.users`, which beats every allow rule including a role grant.
2. Direct messages, against `direct_messages.allow`.
3. The guild entry, where an unlisted guild is refused outright.
4. The member, as the union of `users` and `roles`.
5. The channel, or its parent when the summon arrived in a thread.

Steps one through four decide from the Gateway payload already in memory, so a
guild, member, or channel outside the policy costs nothing. Only an unseen
thread reaches a Discord lookup, and those are bounded separately.

## Roles

`roles` scales in a guild whose membership the operator does not control.
Listing users makes every new person a config change plus a rollout, while a
role grant covers members nobody enumerated, free, because
`Message.Member.Roles` arrives on the Gateway payload. Users and roles are a
union, so either alone is a complete grant.

## Failing closed

A missing file, an unparsable file, an unknown field, a non-snowflake ID, a
duplicate guild, a guild that allows no channel, and a guild that allows no
member are all startup failures. Absent configuration never widens the surface.
`ward exec policy-check` validates the tracked reference copy and validates
`SIRENS_ECHO_ACCESS_POLICY` when set. The image build runs the same check, and
`ward exec test` proves it passes with only the files the build context carries.

## Without the file

A deployment that sets no `SIRENS_ECHO_ACCESS_POLICY` gets the equivalent
synthesized from `DISCORD_CHANNEL_ID`, `DISCORD_GUILD_IDS`, and
`SIRENS_ECHO_DISCORD_DM_ENABLED`. One runtime representation either way keeps
the gate single-pathed, and adopting the file replaces those three variables.

## Denials in telemetry

`sirens_echo.access.checks` carries a closed-set `reason`: `allowed`,
`denied_blocked`, `denied_dm`, `denied_guild`, `denied_channel`, and
`denied_member`. No identifier reaches a label.

See [contexts](sirens-echo-contexts.md), [admission](sirens-echo-admission.md),
and [configuration](sirens-echo-config.md).
