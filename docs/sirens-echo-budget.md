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

## Time bounds

Tokens are one budget and the clock is another. Each MCP phase carries its own
ceiling, because each fails differently: ten seconds to connect, fifteen to list
tools, resources, and prompts, and forty-five for one tool call.

The call bound is the one that had been missing. Without it a tool call was
bounded only by the turn, so a server that never answered spent the whole turn
budget and left nothing to report the failure with. A 180 second hang against a
180 second turn timeout is that shape, and it reached the member as silence.

Six tool rounds still fit inside the turn budget, so the turn timeout stays the
outer bound. The call bound only stops one call from spending all of it.

## See also

See [admission control](sirens-echo-admission.md) and
[the service](sirens-echo.md).
