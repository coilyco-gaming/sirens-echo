# Why a check refused a reply

A refused reply names the check that refused it. It did not say what that check
saw, and the sentence saying so was generated and then thrown away.

`ValidateGrounding` produces `model invented channel #general`. Nothing carried
it: the span status was the cataloged `Response validation failed.`,
`response.check.refused` logged only `check` and `reply_bytes`, and
`turn.stage.failed` named the stage rather than the cause. On sirens-echo#794
that one sentence was the entire answer, and reaching it took four steps and a
source read. See sirens-echo#795.

## What carries it now

`response.validate` carries `response.check.reason`, and
`response.check.refused` logs the same sentence under `refused`. That matches
`model.response.repair` on the completion path, which already records what it
refused rather than only that it happened.

## The reason is not an exception field

`MarkSpanError` still takes only a catalog code. `exception.type`,
`exception.message`, `error.stage`, `error.outcome`, and the span status stay
the fixed cataloged wording, so grouping and alert filters are unmoved and no
runtime data reaches them. See [the exception taxonomy](sirens-echo-exceptions.md).

The reason is an ordinary span attribute and an ordinary log field beside it.
Adding a rule needs no catalog entry and no reviewed increase to the exception
cardinality bound.

## The one reply fragment in telemetry

Every refusal sentence is service-authored except one. `grounding.invented_channel`
names the `#token` the model wrote, and it is deliberate: the token separates a
channel missing from supplied context, which is a prompt problem, from a pure
hallucination, which is not. Without it the rule name says a channel was
invented and no reading of the trace says which.

It is bounded twice. `channelPattern` already constrains the shape to
`#[A-Za-z_][A-Za-z0-9_-]*`, so it is a token rather than prose, and it is
truncated at 64 runes, so a long hallucination cannot enlarge every refusal
record. An invented channel names no real channel and carries no member content.

The identifier check stays the counterexample. Its error names the class and
never the value, because the value is the thing being kept out of a log.

## See also

- [turn stages](sirens-echo-turn-stages.md) - the closed set of check names.
- [grounding](sirens-echo-grounding.md) - the four rules behind that family.
- [observability](sirens-echo-observability.md) - the surrounding log contract.
