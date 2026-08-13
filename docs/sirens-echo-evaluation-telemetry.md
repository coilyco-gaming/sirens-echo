# Evaluation telemetry is its own service

An evaluation run exports to the same receiver as the deployment. It reports as
`sirens-echo-eval`, and that separation is load-bearing rather than tidy.

## What it was

The eval binary built its telemetry config as a literal and never set
`InstanceName`, so it resolved to the `sirens-echo` default. One binary serves
both profiles, so `eval-echo`, `eval-deep`, `board-deep`, and `rate-deep` all
reported as the Echo deployment.

`service.name = sirens-echo` was therefore not one service. It was the
production deployment plus every evaluation run of both profiles.

That is worse than an untidy label. `eval-deep` runs five times, `board-deep`
repeats each case five times inside one run, and `rate-deep` runs each case its
own declared number of times. Measured over 24 hours it was 898 spans against
169 real turns, so most of what the production service appeared to be doing was
evaluation.

Every number taken from that service was wrong in the same direction and none
of them looked wrong: error rate, latency, token spend, and cache-hit ratio all
read as plausible figures.

## Why nobody caught it

`sirens-deep` looked clean at almost exactly one lookup per turn. It looked
clean because its evaluation traffic was not missing, it was being billed to
the other service. A contaminated service and a clean one is a much less
alarming shape than two contaminated ones, so the clean one reassured rather
than prompting a question. See
[sirens-echo#533](https://forgejo.coilysiren.me/coilyco-gaming/sirens-echo/issues/533).

## What is still true

An evaluation run opens no `community.turn`, because it has no turn. Its
`mcp.tools.list` spans are therefore roots whose traces contain only
themselves. That is honest now that they are on their own service, and it is
still not useful. Giving a run its own root span is an open design question.

The two evaluated profiles are not distinguished from each other. Both report
`sirens-echo-eval`, because there is no lowercase slug on a definition to
derive one from.

## See also

- [observability](sirens-echo-observability.md) - the deployment's telemetry.
- [the battery](sirens-echo-battery.md) - what evaluation runs.
