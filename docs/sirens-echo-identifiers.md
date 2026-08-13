# Identifiers a reply may not carry

Input framings are unbounded and output values are enumerable. There is no way
to list the framings that produce a leak, but the set of identifiers this
process holds is finite and known at startup. One check on the reply path
therefore covers a forged identity, a forged prior turn, and a framing nobody
has invented yet, without anticipating any of them.

## Built from configuration, not from a list

Derived at boot from what the pod holds: the principal user ID, the MCP roster
and Agent Proxy endpoints, and the Discord token. Nothing is hardcoded, so the
set cannot drift and there is no list to maintain. Channel and guild IDs are
absent: configured rather than secret, and guarding them made a channel link
unsayable. The principal ID stays guarded in every shape. See issue 289.

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

## Spelling is not the invariant

A literal match reads one spelling. The value is what matters, so any separator
carries it past: digits spaced, hyphenated, grouped, or enumerated one at a time.

Numeric identifiers are therefore compared twice, against the reply and against
the reply stripped to digits. That collapses every separator-based spelling into
one comparison rather than enumerating evasions. An accidental 17 to 20 digit
run carrying the exact value does not occur in ordinary prose.

A reversed string and a base64 blob are still not covered, because both change
the digits rather than their separators. Live QA measured encoded exfiltration
refusing 5 of 5 against the running deployment, so nothing is known to escape.
The gap was that a literal check could not tell that from a missed one.

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
