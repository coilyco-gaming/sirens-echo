# Composing the Sirens Deep identity

Sirens Deep composes an Agent Compose role bundle so it has a real identity.
Sirens Echo composes nothing and stays void of personality. Tracked at issue 98.

## The allowlist is a source declaration

`agent/compose/request.kdl` names the role and its sources.
`agent/compose/aos-public.kdl` enumerates every composed source Sirens Deep may
load, by exact name.

A request `source` takes either `root=` or `declaration=`. Only `declaration=`
is permitted here. With `root=` the **source repository's own**
`.agents/roles.kdl` decides, and `coilyco-bridge/agentic-os-kai`'s creator role
deliberately binds Kai's career, job-search, and LinkedIn context because that
role serves Kai rather than an agent that answers strangers. A declaration
enumerates exactly what this repository admits.

Declarations name exact skills, never globs. A glob silently widens when the
upstream catalogue grows; an enumeration makes widening a visible line in a
diff.

## Where the sources live

Every admitted source is in `coilyco-flight-deck/agentic-os`, which is public.
The house-taste sources were promoted there from the private personal
catalogue for exactly this reason. The rule that decides placement, and the
sources that fail it, are in that repository's `docs/composed-house-taste.md`.

An earlier revision of this page argued that renaming sources to a `kai-`
prefix beat moving them, because "a move only changes which one a glob finds a
skill in". That held for `root=` and globs. With `declaration=` and exact
names, which catalogue holds a source is precisely what matters.

## Build

`scripts/stage-compose-sources.sh` copies each admitted `COMPOSED.md` to
`agent/compose/skills/<name>/SKILL.md`, because a declaration's paths resolve
beneath its own directory. It then composes one bundle per role and verifies
each.

The image runs it against a pinned `AOS_CATALOG_REF` clone, so the bundle is
deterministic and a catalogue change is a reviewed bump. Locally,
`ward exec compose-bundles` uses an `AOS_CATALOG` checkout.

## Runtime

`composed: true` in the definition makes a bundle mandatory. `SIRENS_DEEP_ROLE`
selects which baked bundle loads, so flipping the role needs no rebuild.
A missing or unreadable bundle stops the process: a profile that asks for an
identity never answers without one.

`ValidateSystemPrompt` inverts per profile. A composing profile must carry
`<composed-identity>` and the bundle's own surface. The neutral profile must
carry none of it. The anchors are strings a real bundle contains, not the
historical `<aos-community-bundle>` marker, which appears in no current bundle
and would fail every startup if required.

## Enforcement

`internal/community/compose_test.go` parses the tracked files and fails when the
declaration admits an unreviewed or denied source, uses a glob, declares a
mismatched path, or names a source twice, and when any request source uses
`root=`. The approved set is duplicated in the test on purpose, so widening the
surface changes a test rather than only a config file.

## Identity, not impersonation

The seat carries its own name and pronouns, so the agent has an identity of its
own rather than borrowing a person's. It shares house taste and house style and
never claims to be a specific person. Someone in a guild the operator does not
moderate cannot be left unsure whether they are talking to a human.

## See also

See [the rendered prompt](sirens-echo-prompt.md),
[response profiles](response-profiles.md), and
[configuration](sirens-echo-config.md).
