# The per-role selection record

`agent/rendered/roles/<role>.bundle.txt` records what each baked role selected.
One file per roster role, written by the build where the bundles exist.

## What it carries, and what it deliberately does not

Role skill, model tier, personalities, sources, and the sorted skill set as
`<source>/<skill>`.

No bodies and no digests. The catalogue ref floats on `main` by design, so an
upstream wording edit is not this repository's change to review. A record built
from bodies would go stale on every upstream commit and redden `main` for a
reason nobody here can act on.

What does move it is a role gaining or losing a skill, which is exactly the
change that should be reviewed. Widening the surface stays a visible diff.

## The gate

CI bakes the bundles in the `test` job and fails on a difference there, and the
image build checks the same thing again over the bundles it actually ships. The
second is what stops a bad image; the first is what tells you in seconds rather
than two thirds of the way through a build. See sirens-echo#788.

A record drifts most often because the branch was cut before a composed-sources
change landed on `main`, not because anything on the branch moved. The merge is
the remedy in that case, and the failure names it first.

Loading the bundles renders and validates each role's prompt on the way, which
is the other half. A bundle that failed to compose would otherwise ship as a
quietly neutral agent, and a bundle filed under the wrong role slug would make
`SIRENS_ECHO_ROLE` select the wrong identity with nothing downstream to catch
it. Both fail the build.

Prompt sizes are printed per role and never gated. That is the early warning
this record was asked for, without the false alarm a byte-exact gate would
produce against a floating catalogue.

## Commands

```sh
just role-drift-check     # bake and check in one step, the gate CI runs
just compose-bundles      # bake into agent/bundles, needs AOS_CATALOG
just role-snapshot        # rewrite the records from those bundles
just role-snapshot-check  # fail on drift, over bundles already baked
```

`role-drift-check` clones the catalogue at `AOS_CATALOG_REF` when `AOS_CATALOG`
names no checkout, which is how it runs in CI. It bakes to a scratch directory
and removes it, because pre-commit walks the filesystem and a baked bundle is a
tree of skill files those hooks then read as this repository's own.

`role-snapshot` and `role-snapshot-check` need bundles already baked, so
pre-commit runs neither and stays hermetic on a machine with no catalogue
checkout. The tracked prompt snapshots
under `agent/rendered/` keep rendering the placeholder for the same reason.

## Reviewing a change

A diff here answers "what does this role actually pull". Read it beside the
allowlist diff: `roles.kdl` says what a role may have, and this says what it
got. They differ whenever a glob starts or stops matching upstream, which is the
case worth looking at.

See [composition](sirens-echo-compose.md), [the
prompt](sirens-echo-prompt.md), and [response profiles](response-profiles.md).
