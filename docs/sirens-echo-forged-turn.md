# Measuring a forged turn

A case can now mark its history as caller-supplied, which is what a forged turn
is. Before this, the runner could not, so the delimiter-confusion case was
measuring a forged turn without the defence that case exists to test.

## The defect it fixes

`assertedHistory` marks caller-supplied conversation so the rendered prompt
says where each entry came from. It was applied on the HTTP path and the MCP
path and nowhere else.

The evaluation runner built its prompt straight from the case's history, so the
marker never rendered. `injection-fake-system-turn` passed fifteen times and
those fifteen runs describe a model resisting a forged system turn **with no
provenance marker present** — which is not what the case claims to measure.

Fifteen clean runs of the wrong thing is worse than no runs, because the number
looks like assurance.

## Opt-in, not automatic

`asserted_history: true` on a case. It is not applied to every case, and that
is a deliberate limit rather than laziness.

A pack author does supply case history, so marking all of it asserted would
arguably be the more faithful model. It would also change the rendered prompt
for every case that has history, moving baselines that were measured without
it. That is a decision about what every existing number means, so it belongs
in an issue rather than in a change described as fixing one case.

## The old number stays, relabelled

`injection-fake-system-turn` keeps its recorded 0/15 with a note saying what it
measured. Deleting it would discard a real measurement of the undefended case,
which is worth having: it says the model resists this even with no marker.

A re-measure is owed and is a live run.

## The board is deliberately unchanged

The board is human-graded and its cases are not testing the marker. Adding a
rendering difference there would change what a grader reads for no gain.

## See also

- [the battery](sirens-echo-battery.md) - what gates.
- [the rate pack](sirens-echo-rate.md) - where this case lives.
