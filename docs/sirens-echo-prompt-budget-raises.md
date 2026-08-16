# Prompt budget raises

The append-only ledger behind
[the prompt budget](sirens-echo-prompt-budget.md). Split out of it when the
combined file crossed the documentation cap, which it will keep doing as this
list grows.

## Every raise, and what caused it

Every entry in `promptBudgets` is a ratchet on an observed size.

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

- **Both, by 1951**, to 25772 and 14516, on 2026-08-15. The org relationship
  became a knowledge source on sirens-echo#806, composed by both profiles, so
  the raise is identical again. The largest single raise here, and it buys one
  thing: both agents answer "who do you work for" from a shared source rather
  than each inferring it. A first draft cost 2827 and was cut without dropping
  a fact.

Deep's two profiles are not converging by accident. Deep now carries the same
filing instructions as Echo because it does the same filing, and the budget gap
between the two remains the local policy roots rather than the tracker. Kai
confirmed that on sirens-echo#235: the two get the same settings here, which
`TestBothProfilesShareOneFilingPolicy` now pins.
