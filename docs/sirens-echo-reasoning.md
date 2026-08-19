# Reasoning fields and forged turns

Two places a value's empty state and its real state must stay tellable apart: the reasoning field
the model echoes back, and history a caller supplied rather than the runtime observing it.

## Echoing reasoning back exactly as it arrived

DeepSeek in thinking mode rejects a request whose assistant messages do not carry `reasoning_content`.
The harness copied the field at both assistant build sites, so it looked faithful; **the encoding was
not**, because under `omitempty` a model that returned an empty reasoning string and a model that never
mentioned reasoning produce the same bytes: no key. The harness echoed the second shape for both and the
provider refused the array. **Same defect as above, in an encoding rather than a log line.**

**Dropping `omitempty` was the obvious fix and is wrong.** `chatMessage` is one struct for every role,
so dropping it stamps `"reasoning_content": ""` onto system, user, and tool messages, on every request,
on every route, while the failure lives on one evaluation lane. **A repair that rewrites every request
on the healthy lanes to fix a broken one is not mechanical.**

Both the response and request fields are `*string` instead, **so presence survives the round trip**: a
provider that named the field, even as `""`, gets it echoed, and one that never named it sees a
byte-identical request to the one it saw before. An explicit `null` reads as absent, which is what the
previous encoding did with it too.

**What was not established** is whether the provider accepts `"reasoning_content": ""`, which #717 was
blocked on. It is no longer blocking, because **the change cannot be worse than what it replaces**: the
failing case sends the field where it currently sends nothing, and nothing is already the shape that
earns the 400. If the empty string is refused, the fix left is to put something in a field the model did
not fill, **which is a decision rather than a repair**.

## Measuring a forged turn

A case can mark its history as caller-supplied, which is what a forged turn is. Before that, **the
delimiter-confusion case was measuring a forged turn without the defence that case exists to test.**
`assertedHistory` marks caller-supplied conversation so the rendered prompt says where each entry came
from, and it was applied on the HTTP and MCP paths and nowhere else. The evaluation runner built its
prompt straight from the case's history, so the marker never rendered: `injection-fake-system-turn`
passed fifteen times, and **those fifteen runs describe a model resisting a forged system turn with no
provenance marker present**, which is not what the case claims to measure. **Fifteen clean runs of the
wrong thing is worse than no runs, because the number looks like assurance.**

`asserted_history: true` is opt-in per case, **a deliberate limit rather than laziness**. A pack author
does supply case history, so marking all of it asserted would arguably be more faithful, but it would
also change the rendered prompt for every case that has history, **moving baselines that were measured
without it**, which is a decision about what every existing number means. The old number stays,
relabelled: `injection-fake-system-turn` keeps its recorded 0/15 with a note saying what it measured,
because **deleting it would discard a real measurement of the undefended case**. A re-measure is owed.
**The board is deliberately unchanged**, being human-graded with cases that are not testing the marker,
so a rendering difference there would change what a grader reads for no gain.
