# Agent folders

Everything that pertains to one agent and nothing else lives under
`agents/<name>/`. Everything both agents read stays in `agent/`.

```text
agents/echo/            agents/deep/
  definition.yaml         definition.yaml
  packs/                  packs/
    evaluation.yaml         evaluation.yaml
    rate.yaml               rate.yaml
                            board.yaml
                            rate-fixture.yaml
  probes/                 probes/
  evaluations/            evaluations/
  rendered/prompt.txt     rendered/prompt.txt
```

The file names repeat across agents on purpose. `agents/deep/packs/rate.yaml`
says what `rate-deep.yaml` said, and the folder carries the agent rather than
the filename repeating it.

## What each directory holds

* `definition.yaml` - the profile the harness loads. One per agent, and the name
  is fixed so the runner can derive the rest from it.
* `packs/` - the canonical packs the ward verbs run. Policy-check loads every one
  of these, and a test fails if a tracked pack never reaches it.
* `probes/` - packs run from outside the repository, copied in at run time so a
  committed dataset stays re-derivable. Evidence, not the canonical set, which is
  why they are kept out of `packs/`.
* `evaluations/` - the datasets. A record keeps the paths it was produced with,
  so an old one naming `agent/sirens-deep.yaml` is history rather than a stale
  reference.
* `rendered/prompt.txt` - the tracked prompt snapshot, written beside its own
  definition because every agent's definition is now called `definition.yaml`.

## What stays in `agent/`

`phrases.yaml`, `content-classes.yaml`, `compose/`, `rendered/roles/`, and the
tracker and injection fixtures. These are read by both agents, so filing them
under either one would be a lie about ownership.

## What did not move, and why

`.agents/skills/` is the agentic-os catalog contract rather than this
repository's convention. `check-skills` and `repo-pointer-skills` run against
that path and the workspace `AGENTS.md` calls the directory canonical, so Echo's
and Deep's local skills stay there even though they are agent-exclusive. This is
the one place the rule is deliberately not applied.

`docs/` is repo-prefixed, not agent-prefixed. `sirens-echo-` names the
repository, and the docs describe the shared harness rather than Echo.

## See also

* [Configuration](sirens-echo-config.md) - what a definition declares.
* [Rate provenance](sirens-echo-rate-provenance.md) - what a dataset records.
