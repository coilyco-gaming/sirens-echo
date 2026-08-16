# Validating an access policy offline

`sirens-echo-access-check` answers one question: would this pod accept this
access policy. It reads files and does nothing else, so a sealed CI container
can run it.

```sh
sirens-echo-access-check path/to/access-policy.yaml [...]
sirens-echo-access-check -                      read one policy from stdin
```

Prints `<path>: ok` and what the policy admits, exits 1 with the reason on
stderr for any file that fails, and exits 2 with usage when handed no arguments.

## Deploy holds a ConfigMap, not a policy

**Every file in deploy fails this check when passed as a path**, including the
correct ones. They are Kubernetes manifests with the policy nested under
`data["access-policy.yaml"]`, and the runtime never sees that wrapper because
the ConfigMap projects the key as a file. So deploy extracts the key first:

```sh
yq '.data."access-policy.yaml"' access-policy.yml | sirens-echo-access-check -
```

That split is the boundary rather than a workaround: deploy owns the manifest
format and this repository owns the policy schema.

## Why it exists

Deploy's pre-commit parses the file as YAML, which catches a syntax error and
nothing else. Every check that matters is semantic and lives here, so until
this existed the first thing to evaluate a policy was **pod boot**, after the
ConfigMap was already applied.

Deploy's standing rule is that it never reimplements a parser for a format
another repository owns. It calls the owning tool instead, the way it calls
`mcp-beaver lint` for guardfiles. There was no equivalent here.

## It calls the runtime's own loader

`check` calls `community.LoadAccessPolicy`, which is the same function the
agent calls at startup. That is the whole design.

**A second implementation would be a worse gate than none.** It would pass
policies the pod rejects and reject policies the pod accepts, and the
divergence would appear as a rollout failure that CI called green.

## What it catches

The bound that matters most is a guild opened to every member without a real
per-user rate limit:

```
guild "…" opens to every member: set rate_limit.per_user to a real bound
```

Strict decoding catches the quieter one: a misspelled key like `ratelimit`
fails rather than being ignored, which a plain YAML parse in another repository
cannot see, because the file is valid YAML and the bound is simply absent.
Schema mismatch, unreadable file, and non-snowflake IDs each fail with their
own reason.

## What it prints when it passes

A policy can be entirely valid and still open a guild nobody meant to open, so
a passing run lists each guild's channels, members, roles, and resolved rate
tiers, and says in prose when every member is admitted.

**An unset tier and a disabled one are not the same.** An absent
`rate_limit.per_user` inherits the deployment tier, which this file cannot see,
and `off` removes limiting. They read as `deployment default` and `unlimited`,
because conflating them makes an unbounded guild look bounded.

## See also

- [checking a deploy-owned file](sirens-echo-deploy-owned-files.md) - the rule.

- [the access policy](sirens-echo-access.md) - what the file means.
- [admission control](sirens-echo-admission.md) - what the limits do.
