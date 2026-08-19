# Tuning numbers and feature switches

Every tuning number this package has lives in `internal/community/config.go`, beside what the deployment
supplies. **The generated list of names and defaults is `agent/rendered/knobs.txt`**, written by
`just knobs` from the table itself, and rewritten by the test that reads it, so a moved number
carries its own update.

**Every enabled/disabled switch takes the same treatment**, in `internal/community/featureflags.go`
and generated into `agent/rendered/flags.txt` by `just flags` (#854). One difference: a knob that does
not parse keeps its default, and **a switch that does not parse is fatal**, because a surface silently
off is worse than a refused start.

**A number in the file that uses it is easy to find once you already know which file that is. The
problem is the other direction**: nobody could answer "how many numbers does this service have" or
"which of these are related" without reading everything, and two numbers that must agree could sit in
different files with nothing connecting them.

Here: a number that tunes behaviour - a timeout, cap, bound, retry count, size limit. **Not here**: a
number that is part of a data structure or an algorithm, a cache capacity chosen at a call site, or a
test fixture. **Every one that is here goes through one helper and takes one environment name, so there
is no second tier of numbers a deployment cannot reach.**

Change one here, and **read the neighbours first**: a number in a group usually relates to the others,
**and that relationship is the thing most likely to break**. Where one exists, **prefer writing it down
over restating a value**. The progress cadence is the worked example:
the beat is twice the wait and the long-reply window is the wait plus two beats, so one edit moves all
three and a test holds that derivation while the values follow.

**Collapsing numbers that are close but not equal changes behaviour, sometimes by a large fraction.**
That is a decision rather than a refactor, and it does not belong in a commit described as mechanical.

## A number a definition may override

Most of these are one value for the process. **The model-call ceilings are not, because the two profiles
do not share a substrate**: Echo's route resolves to a 35B model on the daily driver and Deep's resolves
upstream, **so one ceiling that is cheap on Deep is minutes of tower time on Echo**. A definition may
name a `model_budget`, and each field it leaves out takes the value in `config.go`, so **a definition
names only what it changes and one naming none behaves exactly as the defaults did**. Echo names none
(#467). **The declared values stay the defaults rather than becoming dead**, which keeps `config.go` the
answer to "what does this service do if nobody says otherwise".

`turn_model_calls` is the one field in that budget which bounds the turn rather than the completion,
because a turn makes several completions and each of the other fields binds one of them. See
[turn-stages](sirens-echo-turn-stages.md).

A budget is validated at load. Every field is a ceiling, so none may be negative, **a ceiling below the
floor is refused, and so is one the rungs stop short of**: the ladder doubles, so `base` times the step
to the power of the raises has to reach `max`, **because a ceiling that is never applied reads as
granted**. Slack upward is fine. To say never raise, set `max_completion_tokens` equal to
`base_completion_tokens`, **because a raise that is not a raise does not happen**; setting
`budget_raises` to zero does not do it, since zero is unset and takes the default.

## One helper, one behaviour

Each number is declared on one line binding where the package reads it, the name that sets it, and what
it holds without one: `overridable(&defaultQueueTimeout, "SIRENS_ECHO_QUEUE_TIMEOUT", 30*time.Second)`.
A duration takes Go's spelling and a count a plain integer. **Unparsable, zero, or negative applies
nothing, for every name alike**, so a typo leaves the service on its default rather than on a number
nobody chose, **the direction that fails safe when a values file is edited under pressure**. **Silence
there would read as a working override**, so rejected names are reported on the `capabilities` log line
at startup beside the applied ones. **This used to differ by name**: `REQUEST_TIMEOUT`, `QUEUE_TIMEOUT`,
and `SHUTDOWN_GRACE` were parsed a second time to fill a `Config` field, and that second reader refused
a bad value and failed the load - **one name, two readers, two answers**.

**Three numbers are expressions of another and have no name of their own**: `turnProgressEvery` is twice
`turnProgressAfter`, `turnLongReplyAfter` is the wait plus two beats, and `replyAttachmentBytes` is the
scratchpad's per-file limit. They are recomputed **after** the overrides land, because read before, **an
override would move the beat and leave the long-reply threshold on the old number**: the override would
appear to work while the threshold deciding whether a reply gets a thread silently disagreed with it.

**An algorithm's floor is not a number a deployment sizes.** `minNormalizedIDDigits` and
`minEncodedGuardBytes` decide what counts as an identifier rather than how much of one to allow, so they
stay where they are used and no name reaches them, held by `TestNoAlgorithmFloorIsOverridable`.
**Lowering one from a values file would be a way to switch a control off while looking like tuning**,
and a loosened guard is indistinguishable from a configured one from the outside.

**Some of these are Discord's numbers, not ours.** `SIRENS_ECHO_THREAD_PREFILL_PAGE`, the command-shape
bounds, and the reply limit describe what Discord accepts, so raising one past that fails at Discord
rather than here, **arriving as a rejected send rather than a startup error**. They are settable because
everything is, not because moving them is a good idea.

## The guard on where numbers live

`TestEveryTuningNumberLivesInConfigGo` keeps every tuning number in `config.go`. **It reads shape, not
names**: the test parses each non-test file in the package and reports every package-level `const` or
`var` declared with a numeric value, so a number outside `config.go` is a stray unless
`elsewhereByDesign` names it with a reason. A number inside a function body is a local and never reaches
this, as is a string, a struct, a call, and a type.

**Matching names instead would not hold a line.** A pattern over `^(max|min|default)[A-Z]` and a list of
suffixes sees a number only when the author happened to spell it that way, **and a pattern that has to be
widened every time someone names a number a new way is describing the names already used.**

Inverting it costs an exemption for every genuine non-knob, currently seven, **which is the deliberate
half of the trade**: an exemption is a sentence someone had to write and a reviewer can disagree with,
**where a pattern miss produces silence that reads exactly like a pass**. They fall into two families.
**Floors that decide what something is** - `minNormalizedIDDigits`, `minEncodedGuardBytes`, and
`opaqueSecretRunes` set what counts as an identifier or a credential, so lowering one changes what
matches rather than how much of a match is allowed. **File modes** - `scratchPermissions`,
`scratchFilePermissions`, and `workspacePermissions`, because **text-only is enforced by denying the
execute bit, so a deployment that could grant it could undo the property**. `unboundedReply` is neither:
it is the sentinel a transport with no ceiling declares, **the absence of a bound rather than a bound**.

To add one, move the number into `config.go` behind `overridable` and run `just knobs`. **Reach for
`elsewhereByDesign` only when the number is genuinely not a knob, and write the reason as a sentence
rather than a category, because the reason is the whole value of the entry.**
