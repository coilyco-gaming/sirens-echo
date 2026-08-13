# A trusted HTTP caller

`/v1/turn` could not tell one caller from another. This is the first half of
fixing that: authenticating the caller. Sessions and the prompt's principal
assertion are separate, and the order matters.

## What the endpoint faces

Echo is exposed through `ingress-tailscale`. `/v1/turn` is on the tailnet and
has never been on the public internet. That does not make authentication
unimportant — it makes the threat model *which tailnet peer* rather than
*anyone at all*, which is a bounded problem.

## Safe by default

`SIRENS_ECHO_HTTP_TOKEN` supplies the token. **Unset trusts nobody**, which is
exactly what this endpoint did before the token existed. So enabling identity
is a deployment decision and landing the code changes nothing on its own.

A request is trusted when it presents `Authorization: Bearer <token>` matching
the configured value. The comparison is constant time: a check that leaks its
own answer through timing looks like a control and is not one.

## Why not `X-Sirens-Caller`

That header already exists, is self-asserted, and is used as a rate-limit key
so a well-behaved client gets its own budget instead of sharing the anonymous
one. Nothing verifies it and nothing should start to.

It is the trap in this feature. There is a header named like an identity,
carrying a caller-supplied name, already flowing into the turn's requester. The
smallest possible version of "add identity" is to trust it, and that step is
treating an unauthenticated string as a principal. It would pass every test
anyone would think to write, because the plumbing works and the value arrives
where it is expected.

So the trusted input is a different input, and the self-asserted one keeps its
old meaning.

## Sessions come after, not before

A session is a handle to retained conversation. An endpoint that accepts a
session id from an unauthenticated caller and returns that session's history
discloses conversations to whoever guesses an id.

`history_count: 0` is therefore not merely a missing feature. It is the reason
this endpoint is currently safe to expose without authentication, so identity
lands first or the two land together.

## What trust does today

Nothing, beyond a span attribute recording whether the caller authenticated.
That is deliberate: the value of this change is that the distinction exists and
is recorded, and every consumer of it is a separate decision.

The prompt still asserts that an HTTP caller is not the principal. Making that
conditional means rebuilding the system prompt per request, since it is built
once at startup, and that is a cost and a design question rather than a line of
code.

## See also

- [turn identifiers](sirens-echo-turn-identifiers.md) - the precedent for
  putting a caller-shaped value on a span and saying why.
