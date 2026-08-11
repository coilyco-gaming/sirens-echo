# Composing the Sirens Deep identity

Sirens Deep composes an Agent Compose role bundle so it has a real identity.
Sirens Echo composes nothing and stays void of personality. Tracked at issue 98.

## The allowlist

`agent/compose/roles.kdl` names every composed source Sirens Deep may load.
Two rules govern it.

**No private global repositories.** A public repository is fine to globalize.
`coilysiren/lore` and the private overlays are not, and a global naming one of
them fails the suite. Banning globals outright was a proxy that rejected
agent-compose, sirens-echo, and the public profile, all of which are public.

**Tastes and style only.** An organization can own a favorite colour. It cannot
own a person's social accounts, career, or job search. Every entry is
public-safe and mirrored on the public Coilyco website.

## Globs are allowed, denied reach is not

An earlier revision banned globs outright. That was a proxy for the real rule
and it blocked legitimate widening once `personal-preference-social` was
reframed onto the organization and became safe to pull.

The invariant is now direct: no pattern may match a denied source. `kai-*`,
`writing-kai-*`, and a bare denied name all fail, while
`personal-preference-*` passes.

## Enforcement

`internal/community/compose_test.go` parses the file and fails when a source is
outside the reviewed set, when an entry is a glob, or when a global repository
appears. The approved set is duplicated in the test on purpose, so widening the
surface changes a test rather than only a config file.

Each guard is negative-tested. `kai-*`, `kai-career`, `writing-kai-*`,
`global lore`, and an unapproved name each fail the suite.

## Why the writing glob is safe

`writing-*` now means house writing craft in both catalogs. Getting there took
renaming the personal sources to the `kai-` prefix rather than moving them
between repositories, because Sirens Deep composes both catalogs and a move
only changes which one a glob finds a skill in.

`tooling-discord-community-host` is listed separately because the aos creator
role is `writing-*` and cannot reach a `tooling-*` source. It is the closest
match in the catalog to what this agent does.

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
