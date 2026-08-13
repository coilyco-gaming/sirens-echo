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

**Accept both shapes**, or refuse the wrapper with a message that names it. A
reader holding two YAML files cannot see which is which, so the error has to
say.

**Test against a real file.** A fixture written from the format's documentation
describes the mounted shape, which is the one input the checker will never be
handed. Reading one file from deploy before writing the first fixture is one
command.

## See also

- [validating an access policy](sirens-echo-access-check.md) - the checker that
  found this.
