# Identifiers a reply may not carry

Input framings are unbounded and output values are enumerable. There is no way
to list the framings that produce a leak, but the set of identifiers this
process holds is finite and known at startup. One check on the reply path
therefore covers a forged identity, a forged prior turn, and a framing nobody
has invented yet, without anticipating any of them.

## Built from configuration, not from a list

The set is derived at boot from the values the pod actually holds: the principal
user ID, the configured channel IDs, the guild and channel IDs in the access
policy, the endpoints of the MCP roster and the Agent Proxy, and the Discord
token. Nothing is hardcoded, so the set cannot drift as configuration changes
and there is no list for anyone to maintain.

## Admitted by shape, not by membership

A sweep over every configured value would blocklist `8080` and `12`, after which
the service could not say "port 8080" or count to twelve. So a value enters the
set only when its shape says it is an identifier:

- a digit run of 17 to 20 characters, which is a Discord ID. A shorter run is an
  ordinary number a reply may legitimately contain.
- a `host:port` pair. A bare host is a public name and a bare port is an
  ordinary number, so neither is guarded on its own.
- an opaque string of at least 20 characters, which is a credential rather than
  a word.

A configured value that clears none of those tests stays out, however sensitive
it looks.

## The handle is deliberately absent

`coilysiren` is a substring of `forgejo.coilysiren.me`, which tool output
legitimately returns, and a correct refusal frequently quotes the handle back
when someone claims it. A flat match would reject both. `ValidateIdentityClaim`
already owns the handle with a rule that understands the context, so guarding it
here would be a second thing to get wrong about a value that has an owner.

## Forbidden unconditionally

Every value in the set is one no rostered tool returns, so a match is a leak
whether or not the turn called anything. That is why there is no in-turn
exception to reason about. A class that a tool does legitimately return stays
out of the set rather than gaining a conditional rule.

## The rejection names the class

The error says a configured identifier was carried and never the value, because
the whole point is to keep that value out of anything downstream, including a
log. A deployment can confirm the guard is populated from the count recorded at
startup rather than from the contents.

## What this is not

A leak guard, not a fix for why a model discloses under pressure. It bounds the
blast radius rather than removing the pressure, and a rejected turn is still a
failed turn for the member.

## See also

See [identity](sirens-echo-identity.md) for what a reply may not claim to be,
and [caller history](sirens-echo-caller-history.md) for the input-side seam that
made the leak easier to reach.
