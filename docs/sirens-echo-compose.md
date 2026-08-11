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

## What the creator role does and does not reach

The aos `creator` role is `composed-skill "writing-*"`. That reaches the three
`writing-social-*` sources, which are the right ones: adapting to an active
community, running an editorial loop, and handling corrections and
moderation-adjacent moments.

It does not reach `tooling-discord-community-host`, which is `tooling-*`. That
skill is the closest match in the whole catalog to what this agent does, so it
is listed explicitly rather than left to the role glob.

Three `writing-*` sources are deliberately not composed here.
`writing-content-linkedin-video` and `writing-public-repos` are production
formats rather than conversation. `writing-voice-observer-narrator` constrains
its subject to a passive observer that is "not an active agent", which
contradicts an agent that answers and acts.

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
