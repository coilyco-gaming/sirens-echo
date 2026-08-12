# Composing the Sirens Deep identity

Sirens Deep composes an Agent Compose role bundle so it has a real identity.
Sirens Echo composes nothing and stays void of personality. Tracked at issue 98.

## The allowlist is a role graph

`agent/compose/roles.kdl` is the one tracked allowlist, in agent-compose's own
role-graph format, with globs. Every role it declares bakes its own bundle.

`agent/compose/request.kdl` is the second tracked file, and the grammar forces
it: `agent-compose compose` takes a request, and a request source needs a
separate declaration file. Only `declaration=` is permitted. With `root=` the
**source repository's own** `.agents/roles.kdl` decides, and
`coilyco-bridge/agentic-os-kai`'s creator role deliberately binds Kai's career,
job-search, and LinkedIn context because that role serves Kai rather than an
agent that answers strangers.

The declaration itself is generated. Its `path="skills/<name>"` is mechanically
derived, so hand-writing it is boilerplate with a chance of drift.

## Globs stay safe

`cmd/sirens-echo-compose` expands the graph against the pinned catalogue and
fails when a pattern reaches a name in `DeniedComposedSkills`, when a pattern
matches nothing, or when a global resolves to a private repository. A silently
empty selector is how an allowlist rots into permitting whatever an upstream
rename produced, so it is an error rather than a shrug.

## Where the sources live

Every admitted source is in `coilyco-flight-deck/agentic-os`, which is public.
The house-taste sources were promoted there from the private personal
catalogue for exactly this reason. The rule that decides placement, and the
sources that fail it, are in that repository's `docs/composed-house-taste.md`.

## Build

`scripts/stage-compose-sources.sh` runs the generator per declared role, which
copies each admitted `COMPOSED.md` to `agent/compose/skills/<name>/SKILL.md` and
writes the declaration, because a declaration's paths resolve beneath its own
directory. It then composes and verifies one bundle per role.

The staged tree, the generated declaration, and the per-role requests are build
output. The script removes them on exit and a test fails if any is committed.

The image runs it against a pinned `AOS_CATALOG_REF` clone, so the bundle is
deterministic and a catalogue change is a reviewed bump. Locally,
`ward exec compose-bundles` uses an `AOS_CATALOG` checkout.

## Runtime

`composed: true` in the definition makes a bundle mandatory. `SIRENS_DEEP_ROLE`
selects which baked bundle loads, so flipping the role needs no rebuild.
A missing or unreadable bundle stops the process: a profile that asks for an
identity never answers without one.

`ValidateSystemPrompt` inverts per profile. A composing profile must carry
`<composed-identity>` and the bundle's surface; the neutral profile must carry
none of it. The anchors are strings a real bundle contains, not the historical
`<aos-community-bundle>` marker, which appears in no bundle today.

## Enforcement

`internal/community/compose_test.go` expands the tracked graph and fails when a
pattern reaches a denied source, matches nothing, or globalizes a private
repository, and when a request source uses `root=` or build output is tracked.
The deny list lives in `composepolicy.go` beside the expander, so the build and
the suite enforce one list rather than two copies.

## Identity, not impersonation

The seat carries its own name and pronouns, so the agent has an identity of its
own rather than borrowing a person's. It never claims to be a specific person.

## See also

See [the rendered prompt](sirens-echo-prompt.md),
[response profiles](response-profiles.md), and
[configuration](sirens-echo-config.md).
