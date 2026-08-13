# What a reply in another language is still checked for

Every reply validator is either a list of English words or a match on a value. A
translation changes nothing about the second kind and defeats the first kind
entirely, so a non-English reply ships with part of its guarantees and no sign
of which part. This page is that list. See issue 253.

Nothing here decides whether the service should answer in another language.
That is issue 298, and this is the price list for it.

## What survives a translation

| Validator | Why |
| --- | --- |
| `ValidateNoToolCallMarkup` | Matches delimiters, not words. |
| `checkHandleEcho` | Matches the configured handle by value. |
| `checkUserIDEcho` | Matches the ID by value, digits, and encodings. |
| `ValidateGrounding` | The invented-channel half matches a `#token`. |
| `ValidateNeutralStyle` | The emoji and exclamation halves read characters. |

That is most of the security-relevant set. A principal disclosure, a leaked ID,
and unparsed tool-call markup are caught in any language, because none of them
were ever detected by reading English.

## What lapses

| Validator | What goes unchecked |
| --- | --- |
| `ValidateGrounding` | Every action claim. An invented filing is not caught. |
| `ValidateSelfAttributedClaim` | The service naming its own tracker write. |
| `ValidateIdentityClaim` | Claiming to be a person, denying being an agent. |
| `ValidateNeutralStyle` | First person, social openings, personality framing. |
| `checkUserIDEcho` | Spelled-digit evasion, which reads English number words. |

The grounding row is the serious one. An ungrounded claim of a completed write
is the defect behind issues 206, 209 and 211, and the whole defence is a list of
English verbs.

The neutral-style rows mean the profile's promise of impersonal output holds in
English and nowhere else, so a translated reply can open with a greeting and
speak in first person while passing every gate.

## Why the fix is not more word lists

A list per language rots the moment a language is added and nobody remembers to
extend it, and the rot is silent: the check passes. Detection is worse. A
misdetected reply runs a check it should never have run, and a grounding failure
routes straight to `failTurn` with no repair, so a false positive costs a member
their answer.

The reachable direction is the one the surviving column already shows. A check
anchored on a value rather than a word needs no language work at all, and where
a guarantee can be expressed that way it should be.

## What holds this page honest

`validatorlanguage_test.go` records a reach for every reply validator and fails
when a new one is added without that decision. The surviving rows are proved by
running each validator against a French reply carrying the defect, rather than
asserted. The lapsing rows are pinned the same way, each with its English
control, so a row that starts passing reports itself instead of going unnoticed.

## See also

- [Grounding a filing claim](sirens-echo-grounding.md) - the action claims.
- [Response profiles](response-profiles.md) - what neutral promises.
