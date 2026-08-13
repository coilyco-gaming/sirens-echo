# Validating an access policy offline

`sirens-echo-access-check` answers one question: would this pod accept this
access policy. It reads files and does nothing else, so a sealed CI container
can run it.

```sh
sirens-echo-access-check path/to/access-policy.yaml [...]
```

Prints `<path>: ok` per file, exits 1 with the reason on stderr for any file
that fails, and exits 2 with usage when handed no arguments.

## Why it exists

Deploy applies `access-policy.yml` as a ConfigMap during rollout. Its
pre-commit parses the file as YAML, which catches a syntax error and nothing
else. Every check that matters is semantic and lives here, so until this
existed the first thing to evaluate a policy was **pod boot** — after the
ConfigMap was already applied.

Deploy's standing rule is that it never reimplements a parser for a format
another repository owns. It calls the owning tool instead, the way it calls
`ward-mcp lint` for guardfiles. There was no equivalent here.

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

Strict decoding catches the quieter one. A misspelled key like `ratelimit`
fails rather than being ignored, which is the failure a plain YAML parse in
another repository cannot see — the file is valid YAML and the bound it was
meant to set is simply absent.

Schema mismatch, unreadable file, and non-snowflake IDs all fail with their
own reason.

## See also

- [the access policy](sirens-echo-access.md) - what the file means.
- [admission control](sirens-echo-admission.md) - what the limits do.
