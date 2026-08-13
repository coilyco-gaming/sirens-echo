# Trace lookup

A member handed a trace id should be able to ask about it in the channel they
were handed it in. This document covers the half that exists, the half that
does not, and the access rule the missing half needs before it is built.

## What exists

Recognition. A turn is a trace lookup when both halves are present:

- the word `trace`, on its own word boundary
- a trace id: 32 hex characters, also on word boundaries

The id can be typed by the member or carried by the message they replied to.
The replied-to case is the common one, because the id usually arrives in a
harness notice and the member simply answers it. A typed id wins over a quoted
one, since a member who types an id has named a different turn from whichever
they happen to be replying under.

Both halves are required, in both directions. The word alone cannot name a
turn, and guessing which one would be worse than not answering. An id alone is
someone quoting a hash, and reading that as a telemetry request is the false
positive the keyword exists to prevent.

Case is normalised, because a member pasting from a console is not obliged to
preserve the lowercase form the wire uses.

## What does not exist

Retrieval. This service has no tracing backend in its roster. The grant is
deploy-owned and tracked in
[issue 278](https://forgejo.coilysiren.me/coilyco-gaming/sirens-echo/issues/278)
and
[deploy 359](https://forgejo.coilysiren.me/coilyco-bridge/deploy/issues/359).

Until then a recognised lookup is recorded as `turn.trace.requested` with
`served: false`, and the member's turn proceeds normally. The record exists so
the demand is measured before the fetch is built, rather than assumed.

## The access rule the fetch needs

A trace id is not a secret, but it is also not scoped to whoever pastes it. A
turn span carries the author's account id, the channel, and the message id, so
a member pasting another member's id would be asking this service to read that
member's identifiers out in a public channel.

Three rules are plausible:

1. **Own turns only.** The trace must resolve to a turn whose account id
   matches the asker. This covers the common case, which is a member asking
   about the failure they just hit.
2. **Anyone, summarised.** Stage, outcome, and timing. Never identifiers.
3. **Anyone, everything.** Defensible in an operator channel, not in `#bots`.

The seam is built toward the first, because a narrow rule can be widened by a
decision and a wide one can only be narrowed by an incident. The choice is the
director's and is recorded on the issue.

## Coupling worth knowing about

The detector reads the notice this harness emits. A test asserts the harness's
own notice is still parseable by it, so a change to the notice shape fails in
the test rather than quietly in production.

## See also

- [notices carry the trace](sirens-echo-notice-trace.md) - where the id comes
  from. The turn identifiers a fetched trace would expose are described in
  `AGENTS.md` under Safety.
