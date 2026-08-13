# What a turn costs

`promptBudgets` ratchets the system prompt. Every request also carries a turn
context, and until now nothing measured it.

## The hole this closes

The rendered snapshots are the fixed half of a request. The variable half is
the conversation window plus the member's message, and it is large:

| Carrier | Cap | Count |
| --- | --- | --- |
| history author | 80 runes | `max_context_messages` |
| history content | 1000 runes | `max_context_messages` |
| current message | 2000 runes | 1 |

At the tracked window of 12 a worst-case turn assembles **15,248 bytes**,
against an Echo system prompt of roughly 19,900. So the unmeasured half was
about 43% of the whole request.

The practical consequence was that raising `max_context_messages` from 12 to
30, or the per-entry cap from 1000 to 3000, cost nothing in any tracked budget
while being paid on every turn forever. Both are now failures with a number
attached.

## Why the window is 12

Issue 194 records the decision: a fixed window plus fetch-on-demand backfill,
with the window size left open. It was already 12 in both definitions, so this
records the reasoning rather than choosing a value.

The fixed window should be the cheapest thing that makes ordinary continuity
work. Raising it buys context for every turn, including the self-contained ones
that need none, which is the cost the hybrid model exists to avoid. A turn that
genuinely needs more should reach for it through backfill, where the cost falls
only on the turns that ask.

## What the test does

`worstCaseTurn` feeds entries far larger than the caps, so it measures what the
caps permit rather than what a caller happened to send. The window is read from
the tracked definitions rather than hardcoded, so measuring at a smaller window
than the deployment uses is not possible.

A second check keeps the truncation itself honest. The budget is a ceiling, so
a cap that stopped truncating would not fail it on its own. That check asserts
the ellipsis is still there and the message cap still bites.

## Raising the budget

Do it deliberately and say why in the commit, the same rule the prompt budget
follows. The number is the worst case, not the typical one: a real Discord
turn is far smaller, because most messages are short. The ceiling exists so a
change to the caps is a decision rather than a side effect.

## What this does not measure

Token count. Bytes are a proxy, and a tokenizer would give a truer number for
the thing that actually costs money. Bytes are checkable offline with no model
and no network, which is what makes this a test rather than a report.

It also does not measure tool results, which enter the conversation later in
the turn and are bounded separately by the 8 KiB spill.
