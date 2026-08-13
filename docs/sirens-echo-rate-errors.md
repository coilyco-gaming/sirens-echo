# Errors and the decimated sample

A pass rate is computed over attempts that **returned content**. Errors are
excluded from the denominator, which is correct: a 502 or an empty completion is a
fact about the substrate, not a behaviour. Counting one as a behavioural failure
would corrupt every rate.

The consequence is that **a clean verdict can rest on far fewer runs than the case
declared**.

## The observation

A case with `runs: 5` reported:

```
passed: 2    attempts: 2    errors: 3
```

`2/2 passed`, read as 100 percent. Three attempts returned empty content after the
proxy exhausted a 3600 token budget and escalated twice. So the pass rate was 100
percent of what was scored and 40 percent of what was asked for, and nothing in
the headline said so. Filed as
[sirens-echo#325](https://forgejo.coilysiren.me/coilyco-gaming/sirens-echo/issues/325).

## What the instrument now says

- the breach line names how many declared runs errored and were excluded
- a `rate.sample.decimated` warning is logged for **any** case with errors, so a
  case that passed surfaces it too, on stderr rather than in the dataset stream

So `errors` is not a field to skim past. Read it beside `attempts` and `runs`.

## What this does not do

**It does not make the rate more reliable.** A behaviour rate over 2 attempts is a
weak measurement whether or not the reader can see that 3 attempts vanished. The
promotion arithmetic in [the rate pack](sirens-echo-rate.md) applies to the
*scored* attempts, not to the declared `runs`, and a decimated sample is weaker
than its `runs` field suggests.

**No error ceiling gates anything.** Failing a verdict on an error rate needs
somebody to decide what rate is acceptable, which is a live-operations judgement
rather than a measurement one. There is no evidence for a defensible default, so
inventing one would be the certifying-rather-than-measuring failure in a new
place.
