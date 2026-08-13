# Checking a file deploy owns

Two of Echo's configuration files live in the deploy repository as Kubernetes
ConfigMaps. Anything here that validates one has to know that, because the pod
never sees the shape the repository stores.

```
services/sirens-echo/deploy/access-policy.yml   kind: ConfigMap
services/sirens-echo/deploy/mcp-roster.yml      kind: ConfigMap
```

Kubernetes mounts the value under `data:` as a file, so the **runtime loader
receives a bare document and is right to expect one**. An offline checker reads
the repository file, which is the wrapper. The two inputs are different and both
are real.

## It failed twice, in opposite directions

`LoadAccessPolicy` uses `KnownFields(true)`, so a ConfigMap's `apiVersion`,
`kind`, `metadata` and `data` are unknown and it **errors**. The gate built on
it rejected every policy deploy has, while passing every fixture.

`LoadMCPRoster` had no such strictness, so the same wrapper parsed, `mcpServers`
was simply absent, and it returned **zero servers and no error**. An empty
roster is a legitimate state, so a wrong path and a deliberate no-tool
deployment looked identical.

**The loud one cost half a day of noticing. The quiet one cost a deployment its
tools with nothing in a log.** Neither loader was wrong for the pod. Both were
wrong for a checker.

## What a checker owes

**Read the inner document, and let deploy extract it.** This is what the access
gate shipped, and it is the seam:

```sh
yq '.data."access-policy.yaml"' access-policy.yml | sirens-echo-access-check -
```

Unwrapping `data:` inside the checker is the tempting alternative and it is
wrong twice. It puts Kubernetes knowledge in the repository that does not own
k3s, and it makes the checker accept a shape the runtime never sees, so the
checker and the pod stop agreeing about what a valid file is. That drift is the
whole reason a checker calls the runtime's own loader.

**Refuse the wrapper with a message that names it.** A reader holding two YAML
files cannot see which is which, so the error has to say.

**Test against a real file.** A fixture written from the format's documentation
describes the mounted shape, which is the one input the checker will never be
handed. Reading one file from deploy before writing the first fixture is one
command.

## Do not iron out how the loaders differ

`LoadAccessPolicy` narrows a schema this repository owns, so strict decoding is
free. `LoadMCPRoster` cannot: the `mcpServers` shape is shared with mcporter,
Claude Code and Codex, and strict decoding would break the first time one of
them adds a key. It refuses an empty roster instead.

Same wrong file, two refusals, both correct for their format.

## See also

- [validating an access policy](sirens-echo-access-check.md) - the checker that
  found this.
