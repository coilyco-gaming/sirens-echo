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

The image build regenerates every record and fails on a difference, so a silent
selection change stops the image rather than shipping.

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
ward exec compose-bundles      # bake into agent/bundles, needs AOS_CATALOG
ward exec role-snapshot        # rewrite the records from those bundles
ward exec role-snapshot-check  # fail on drift
```

Both record commands need baked bundles, so pre-commit runs neither and stays
hermetic on a machine with no catalogue checkout. The tracked prompt snapshots
under `agent/rendered/` keep rendering the placeholder for the same reason.

## Reviewing a change

A diff here answers "what does this role actually pull". Read it beside the
allowlist diff: `roles.kdl` says what a role may have, and this says what it
got. They differ whenever a glob starts or stops matching upstream, which is the
case worth looking at.

See [composition](sirens-echo-compose.md), [the
prompt](sirens-echo-prompt.md), and [response profiles](response-profiles.md).
