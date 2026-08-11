# Composing the Sirens Deep identity

Sirens Deep composes an Agent Compose role bundle so it has a real identity.
Sirens Echo composes nothing and stays void of personality. Tracked at issue 98.

## The allowlist

`agent/compose/roles.kdl` names every composed source Sirens Deep may load.
Two rules govern it.

**No global repositories.** The host profile declares `global profile` and
`global lore`. Either would put private operating context into an agent that
answers strangers in a guild the operator does not moderate.

**Tastes and style only.** An organization can own a favorite colour. It cannot
own a person's social accounts, career, or job search. Every entry is
public-safe and mirrored on the public Coilyco website.

## Exact names, never globs

`personal-preference-*` would silently include `personal-preference-social`,
which is the one preference source that must not compose here. Listing sources
by exact name makes every addition a reviewed line in a diff.

## Enforcement

`internal/community/compose_test.go` parses the file and fails when a source is
outside the reviewed set, when an entry is a glob, or when a global repository
appears. The approved set is duplicated in the test on purpose, so widening the
surface changes a test rather than only a config file.

Each guard is negative-tested: adding `personal-preference-social`, a
`personal-preference-*` glob, or `global lore` each fails the suite.

## Identity, not impersonation

The seat carries its own name and pronouns, so the agent has an identity of its
own rather than borrowing a person's. It shares house taste and house style and
never claims to be a specific person. Someone in a guild the operator does not
moderate cannot be left unsure whether they are talking to a human.

The seat colour and the organization palette do not collide: the accent belongs
to the seat, purple and black belong to Coilyco.

## See also

See [the rendered prompt](sirens-echo-prompt.md),
[response profiles](response-profiles.md), and
[configuration](sirens-echo-config.md).
