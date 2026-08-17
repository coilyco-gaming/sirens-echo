# The guard on where numbers live

`TestEveryTuningNumberLivesInConfigGo` is what keeps every tuning number in
`internal/community/config.go`. This page says how it decides, and why it
decides that way rather than the obvious way.

## It reads shape, not names

The test parses each non-test file in the package and reports every
package-level `const` or `var` declared with a numeric value. A number outside
`config.go` is a stray unless `elsewhereByDesign` names it with a reason.

A number inside a function body is a local and never reaches this. So is a
string, a struct, a call, and a type. `4 * 1024` and `10 * time.Second` are
numbers; `0o700` is one too.

## Why not match the name

The first version did. It tested a stray's name against `^(max|min|default)[A-Z]`
and a list of nine suffixes, so it saw a number only when the author happened to
spell it that way.

That failed twice on the sweep it was written for. Four of the seven numbers
recovered on sirens-echo#829 were invisible until the pattern was widened, and
`mcpsReplyBudget` stayed invisible after the widening because it is neither a
`max*` nor any of the nine. It was a send budget under Discord's interaction
bound, settable by nobody, and the test was silent on it for as long as it
existed.

A pattern that has to be widened every time someone names a number a new way is
not holding a line. It is describing the names already used.

## The trade

Inverting it costs an exemption for every genuine non-knob, and there are
currently seven. That is the deliberate half of the trade: an exemption is a
sentence someone had to write and a reviewer can disagree with, where a pattern
miss produces silence that reads exactly like a pass.

The exemptions fall into two families:

* **Floors that decide what something is.** `minNormalizedIDDigits`,
  `minEncodedGuardBytes`, and `opaqueSecretRunes` set what counts as an
  identifier or a credential. Lowering one changes what matches, not how much
  of a match is allowed, so a deployment must not reach them.
* **File modes.** `scratchPermissions`, `scratchFilePermissions`, and
  `workspacePermissions`. Text-only is enforced by denying the execute bit, so
  a deployment that could grant it could undo the property.

`unboundedReply` is neither: it is the sentinel a transport with no ceiling
declares, so it is the absence of a bound rather than a bound.

## Adding one

Move the number into `config.go` behind `overridable`, and run `just knobs` so
the reference regenerates. Reach for `elsewhereByDesign` only when the number
is genuinely not a knob, and write the reason as a sentence rather than a
category, because the reason is the whole value of the entry.

## See also

- [tuning numbers](sirens-echo-tuning.md) - why one file holds them.
- [tuning a deployment](sirens-echo-tuning-overrides.md) - the override helper.
- [the reference](sirens-echo-knobs.md) - the generated list of names.
