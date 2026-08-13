# Turn stages

A turn's wall clock should be attributable to named stages. Two intervals were
not, and both of them were where the failures decide.

## The gap, and what was in it

Between the last `model.response` and `turn.reply.ready` a turn spent seconds
under no span at all. One measured turn spent 9.02 seconds there, 43% of its
duration, and a successful one spent 4.34. See sirens-echo#652.

**It is the settle wait**, and `turn.progress.settle` now covers it. See
[reply progress](sirens-echo-progress.md).

Worth knowing rather than only worth measuring: a member's answer can be held
for up to 10 seconds after it is ready. Whether that trade is right is a
separate question from whether it is visible.

## Which check refused a reply

`response.validate` carries `response.check`, naming the check that refused, or
`none` when the reply passed. The value is present on every turn, because
absence of an attribute is not something a reader should have to interpret.

The closed set, in the order they run:

```
parse                  tool_call_markup       grounding
self_attributed_claim  identifier_disclosure  identity_claim
response_style
```

Order is the contract, so the checks are a slice rather than a chain of
conditions. The first to refuse is the one named.

Before this, a rejected reply produced `turn.stage.failed` with
`error_type: model_failed`, which reads as the backend failing. It was the
harness refusing the model's output, and nothing said which rule did it. See
sirens-echo#651, where two correct answers were discarded and reported as a
backend outage.

The completion layer has its own contract check, which runs before any of
these and refuses on parse or response style. `model.response.repair` records
what it refused rather than only that it happened, and `model.response.refused`
records the reason a turn gave up. That path reports `stage=model`, which is
true about the code and false about the world.

**Naming a check does not change any verdict.** Whether a rule is right, and
whether exhausting the repair path should report an outage, are decisions on
651 and sirens-echo#396.

## Delivery is recorded, not inferred

`turn.reply.delivered` is emitted when a send returns, so a delivered reply can
be counted. Previously only the failure was recorded, so delivery had to be
inferred from the absence of an error, and an investigation into replies that
never arrived could not tell a silent success from a silent loss.

## See also

- [why a ready reply did not land](sirens-echo-delivery-failures.md) - what a
  failed send records.
- [reply progress](sirens-echo-progress.md) - the line the settle wait protects.
