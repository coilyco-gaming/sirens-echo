# Tuning numbers

Every tuning number this package has lives in `internal/community/config.go`,
beside what the deployment supplies. This document says why they are in one
place and what to do when one needs to change.

## Why one file

A number in the file that uses it is easy to find once you already know which
file that is. The problem is the other direction: nobody could answer "how many
numbers does this service have" or "which of these are related" without reading
everything. Two numbers that must agree could sit in different files with
nothing connecting them, which is how the progress cadence came to be three
constants whose relationship existed only in whoever remembered it.

`TestEveryTuningNumberLivesInConfigGo` holds the arrangement, which seven
numbers had drifted out of, and a knob-shaped number that is not one takes a
named exemption. See sirens-echo#829.

## What lives here and what does not

Here: a number that tunes behaviour. A timeout, a cap, a bound, a retry count,
a size limit.

Not here: a number that is part of a data structure or an algorithm, a cache
capacity chosen at a call site, or a test fixture. Those are not knobs.

Every one that is here goes through one helper and takes one environment name,
so there is no second tier of numbers a deployment cannot reach. See [tuning a
deployment](sirens-echo-tuning-overrides.md).

## Changing one

Change it here, and read the neighbours first: a number in a group usually has
a relationship to the others in it, and that relationship is the thing most
likely to break.

Where a relationship exists, prefer writing it down over restating a value.
The progress cadence is the worked example: the beat is twice the wait and the
long-reply window is the wait plus two beats, so one edit moves all three and
a test pins both the values and the derivation.

## A number a definition may override

Most of these are one value for the process. The model-call ceilings are not,
because the two profiles do not share a substrate: Echo's route resolves to a
35B model on the daily driver and Deep's resolves upstream. One ceiling that is
cheap on Deep is minutes of tower time on Echo.

So a definition may name a `model_budget`. Each field it leaves out takes the
value in `config.go`, so a definition names only what it changes and a
definition naming none behaves exactly as the defaults did. Echo names none.
See sirens-echo#467.

The declared values stay the defaults rather than becoming dead, which keeps
this file the answer to "what does this service do if nobody says otherwise".

A budget is validated at load. Every field is a ceiling, so none may be
negative. A ceiling below the floor is refused, and so is one the rungs stop
short of: the ladder doubles, so `base` times the step to the power of the
raises has to reach `max`. A ceiling that is never applied reads as granted.

Slack upward is fine, and the ceiling binds where it is named. To say never
raise, set `max_completion_tokens` equal to `base_completion_tokens`, because a
raise that is not a raise does not happen. Setting `budget_raises` to zero does
not do it: zero is unset and takes the default.

## Fewer numbers is a separate job

Collapsing numbers that are close but not equal changes behaviour, sometimes
by a large fraction. That is a decision rather than a refactor, and it does not
belong in a commit described as mechanical. Candidates are gathered on the
issue for a decision, not applied here.

## See also

- [the budget ladder](sirens-echo-budget.md) - the numbers with the most
  behaviour attached to them.
- [progress cadence](sirens-echo-progress-cadence.md) - the derivation this
  file is meant to encourage.
