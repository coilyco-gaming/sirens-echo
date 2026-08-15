# A turn that thought until it had no room to answer

A reasoning model can spend its whole completion budget on `reasoning_content`
and return an empty `content` with `finish_reason: length`. The turn produced
nothing, so it fails. What it is not is a backend failure: every model call
returned 200, and one capture showed the reply drafted in full inside the
reasoning, cut mid-word in a trailing paragraph.

## Why it needed its own cause

The failure carried no sentinel, so `failureCause` could not see it and it
landed in `stage_failed`. The stage is `model`, so the member read `model
backend unavailable, retry shortly`.

That sends an operator to the model backend, which is working, and tells a
member to retry a question that will fail the same way. It is the same defect
sirens-echo#258 fixed for spent tool rounds and sirens-echo#651 fixed for a
refused reply, arriving from a third direction.

`ErrBudgetExhausted` now marks it, `budget_spent` counts it, and the member
reads `ran out of room to answer, ask for something narrower`, which is the move
that works.

## Two budgets, two causes

`rounds_spent` and `budget_spent` both mean a ceiling this service chose ended
the turn, and sharing one value would be the same collapse one level down. They
are different numbers with different owners: `rounds_spent` is `tool_rounds`,
`budget_spent` is `max_completion_tokens`. A ceiling decision has to be
answerable from the failure series, and it cannot be if both spends look alike.

## What this does not do

It does not stop the turn failing, and it should not. A member asked a question
and received nothing, so a battery that went green here would be worse than one
that goes red for the wrong reason. The change is what the red says.

It does not bound deliberation, raise any ceiling, or make an unanswered
question answered. Those stay with sirens-echo#367, which is a spend decision
rather than a plumbing gap.

## See also

* [Delivery failures](sirens-echo-delivery-failures.md) - the closed cause set
  this joins, and why a stage is not a cause.
* [Budget](sirens-echo-budget.md) - the ladder, and what a raise costs.
* [Notices](sirens-echo-notices.md) - why a member-facing phrase is fixed.
