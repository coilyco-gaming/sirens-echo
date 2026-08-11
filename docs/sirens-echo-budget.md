# Completion budget

A turn that calls tools re-injects their results into the next request, which
inflates the prompt. Four parallel Eco calls once took a 6k prompt past 47k.

A reasoning model then spent the whole fixed completion budget on
`reasoning_content` and returned empty content with `finish_reason: length`.
The single repair retried at the same budget and hit the same wall, so the turn
failed. That was 20 percent of turns, worst on ordinary member questions,
because the trigger is not difficulty but whether the turn reaches for a tool.

Nothing upstream reported it. A truncated completion is a valid response at the
transport layer, so Agent Proxy and LiteLLM both logged `outcome=ok`.

## Two bounds

**Tool results are capped before re-injection.** Only the copy that re-enters
the prompt is bounded, and the model sees a truncation marker. The full result
is kept for grounding validation, so a bounded result cannot make the runtime
accept an action claim it should reject.

**The budget escalates rather than repeating.** A completion that is truncated
*and* empty raises the budget and retries, from 900 up to a cap of 3600 across
two raises. Beyond that the turn fails with an error naming the truncation
rather than a generic contract failure.

Truncated output that is not empty is a usable answer and does not raise.

## Why both

Raising the budget alone treats the symptom, because a larger prompt finds the
new ceiling too. Bounding results alone leaves any turn that still overruns
failing at a fixed wall. The cause and the symptom need separate bounds.

## See also

See [admission control](sirens-echo-admission.md) and
[the service](sirens-echo.md).
