# Tool call disclosure

A reply that called tools carries a footer naming them. It is the receipt a
reader can see, where the grounding check is a guard only the service can see.

```
Copper ore is 2.4c at Kai's Emporium, cheapest on the server.
I could not confirm current stock.

> 🔨 ✅ `eco.get_market`
> 🔨 📭 `eco.find_trade` — no results
> 🔨 ❌ `eco.get_stores`
```

## Three states, because two would conflate

| Glyph | Meaning |
| --- | --- |
| ✅ | the call returned data |
| 📭 | the call worked and returned nothing |
| ❌ | the call failed |

A failed lookup, an empty result, and a full one read three different ways. A
reply that reported an empty result as a confident zero is the defect this
distinction exists to prevent.

The glyph is a scanning anchor rather than the message, so an empty result also
says `no results` in words. A reader who does not recognise the glyph still
gets the fact.

## Aggregation is consecutive, and never crosses a status

A run of the same tool at the same status collapses to one line with a count.
Any other tool breaks the run, so `A A B A` renders three lines rather than
two and the order the model worked in survives.

A status change breaks it too. A failure is never counted inside a run of
successes, because failures are the point of the footer.

## Bounds

**No tools, no footer.** A refusal calls nothing and stays short.

**Names only, never arguments.** Names come from the roster and are safe.
Arguments can carry member text, and reflecting that back would build a surface
into the data-borne injection vector.

**Service-authored.** The footer is appended after the reply checks rather than
passed through them, alongside issue references, because the harness wrote it
and the checks exist to police what the model wrote.

**Inside the send budget.** A transport with a ceiling declares it, and the
answer is shortened to leave room rather than the footer being truncated away.
A receipt that vanishes on a long reply reads as no tools ran, which is the one
belief this exists to prevent, and it would vanish exactly when the reply is
long and tool-heavy.

**A reference outranks the receipt.** The footer is not budgeted alone. It is
appended with every other service suffix by one step that owns the whole
budget, and it is appended last, so at the ceiling the footer yields and the
link a member can act on survives. Both now survive. See [reply
assembly](sirens-echo-reply-assembly.md).

**Both lanes.** The reply path is shared, so Echo and Deep both disclose. The
verification problem is identical on both.

## Outcome is recorded, not inferred

The classification happens where the call completes. An empty result is
recoverable from the result text afterwards and a failure is not, so waiting
until the reply is assembled would silently lose the state that matters most.

A transport failure never reaches the footer, because it ends the turn instead
of returning a result.

## See also

See [runtime MCP tools](sirens-echo-tools.md) for how a call is bounded and
reported, and [notices](sirens-echo-notices.md) for the harness voice this
footer is written in.
