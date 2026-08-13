# A failed turn cites itself

A turn ending in anything other than success carries a second notice line
naming its trace, so a member's screenshot is a query rather than a report:

> `turn timed out, retry shortly`
> `trace id 3dd883c6becba130e9f8b75e4593a94d`

## Why not a colon

Both lines take the notice alphabet, which admits `[a-z0-9 ,./-]` and nothing
else. `trace ID:` cannot survive it, so the line reads `trace id`. Widening the
alphabet for one label would weaken the property that makes a notice
recognisable at a glance, which is worth more than the punctuation.

## Outside a span

The line is omitted entirely. Not every refusal happens inside a turn span, a
rate-limit shed can fire before one exists, and a blank identifier reads as a
defect rather than as an absence.

## What carries it

Every member-facing failure notice: stage failures, timeouts, rate-limit
cooldowns, queue sheds, and an undelivered reply. A successful reply never
gains one, because the line is what marks a turn that did not succeed.

The value is the turn's own trace, so it joins the same span the metadata logs
carry. An operator pastes it into SigNoz and has the whole turn.

## See also

- [harness notices](sirens-echo-notices.md) - the shape every notice takes.
- [observability](sirens-echo-observability.md) - what a trace joins.
