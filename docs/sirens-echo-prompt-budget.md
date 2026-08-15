# Prompt budget

`TestRenderedPromptsStayInsideTheirBudget` bounds each tracked snapshot in
`agent/rendered/`. The numbers in `promptBudgets` are a ratchet, not a target
and not a claim about the right size.

## Why a budget at all

Every turn ships the whole system prompt. Growth is not a one-off cost, it is a
per-turn cost paid for as long as the profile runs, and it is invisible in a
diff that adds ten reasonable lines to a policy root.

The Echo prompt went from 6918 bytes to 16962 in a single evening, across four
separate changes that were each defensible on their own. Nobody chose 16962.
That is the failure this test exists to prevent: not a large prompt, but a
large prompt that arrived without a decision.

## How to use it

When a change pushes a snapshot past its budget, the test names the file, the
actual size, and the ceiling. Two honest responses:

- Raise the number in `promptBudgets` and say in the commit message why the
  bytes are worth it.
- Trim a policy root instead.

Both are fine. Silently growing is not, and that is the only outcome this
removes.

The budgets carry headroom above the current sizes on purpose. A test that
fails on every ordinary edit trains people to raise the number without reading
it, which converts the ratchet back into a rubber stamp.

## What this does not do

It does not measure cost. A byte count is a proxy for tokens and a poor one
across tokenizers, and it says nothing about the cache behavior tracked in the
prompt-caching issue. If that caching lands, the per-turn cost of a large
prompt falls sharply and these numbers deserve revisiting rather than
defending.

It also does not judge content. A registry of complete URLs is larger than a
template the model fills in, and it is larger on purpose, because a model with
no closed list invents addresses. That trade was made deliberately and the
budget is where it stays visible.

## Raises, and what caused each

Every entry in `promptBudgets` is a ratchet on an observed size. A raise is only
correct when the growth was intended, so each one is recorded here rather than
argued in the diff.

- **Echo, to 21976**, from 21459. Composing the ops role.
- **Deep, to 12260**, from 11600, on 2026-08-15. Deep gained
  `issue_tracker: forgejo`, and naming a tracker renders the filing
  instructions Echo already carried. The whole 863 bytes is that block. No
  policy root was added, and no content was authored for Deep specifically.

- **Both, by 315**, to 22291 and 12575, on 2026-08-15. Kai widened the filing
  trigger on sirens-echo#235 to any unanswerable in-scope question, and added
  two requirements the rule did not carry: one issue per turn, and never filing
  an out-of-scope question. Redundant wording paid part of it; the rest is the
  new rule. The raise is identical on both profiles, which is the shared block
  showing up as arithmetic.

- **Echo, to 23826**, from 22291, on 2026-08-15. Object emoji, sirens-echo#203.
  The whole 1535 bytes is `references/object-emoji.md` plus its pointer, and
  most of that is the five rules rather than a lookup table. A per-item table
  was drafted and cut to one example line, because the model already knows that
  wood is 🪵 and the rules are the part it cannot infer. Deep is unchanged: the
  neutral style check is Echo's, and the social profile never banned emoji.

Deep's two profiles are not converging by accident. Deep now carries the same
filing instructions as Echo because it does the same filing, and the budget gap
between the two remains the local policy roots rather than the tracker. Kai
confirmed that on sirens-echo#235: the two get the same settings here, which
`TestBothProfilesShareOneFilingPolicy` now pins.
