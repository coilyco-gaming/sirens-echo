# Retrying the model call

Only failures that arrive fast are retried. That bound is the whole design and
it comes from the turn ceiling rather than from taste.

```
modelRetryAttempts = 4
modelRetryBackoff  = 250ms, doubling
```

Four attempts, so 250ms + 500ms + 1s of waiting at most. Under two seconds
against a 180 second turn.

## Why fast failures only

`defaultRequestTimeout` bounds the whole turn at 180 seconds, and Echo's p99
turn already sits at that number. A retry ladder built for slow failures does
not fit: five attempts at thirty seconds each is longer than the turn is
allowed to live, so the retry would convert a failure the member could have
been told about into a timeout they wait out.

A connection refused or a 503 returns in milliseconds. Four of those cost less
than the notice they replace.

## What is retried

Availability, and nothing else.

- **transport errors** — no server was reached, so there is no status to read
- **429, 502, 503, 504** — the server saying it cannot serve this now

## What is not

**4xx other than 429.** A 400 is the request being wrong. Retrying it produces
the same 400 four times and delays the notice by the whole ladder. That case is
real here: a prompt-trimming defect upstream returns 400, and a naive retry
would have made it four times slower to report.

**A cancelled or expired turn.** The budget is already gone. Retrying spends
time the turn does not have and the member has stopped waiting for.

## The upstream retries too

Agent Proxy retries independently, twice, at 600 seconds each. A turn that
Echo gave up on at 180 seconds leaves a request running for twenty minutes.

That is not this ladder and this ladder does not change it. It is worth knowing
because the two multiply: anything added here rides on top of an upstream
ladder that is already longer than the turn.

## A refusal is not an outage

A 4xx means the backend answered and refused the request this service built.
Retrying rebuilds the same request, so `rejectedByModel` classifies it and the
member is told the harness built something the model refused rather than that
the backend is unavailable.

`429` and `408` are excluded: those are the two a 4xx can mean that waiting does
fix, so they stay retryable and keep the availability notice.

This mattered under sirens-echo#875, where a malformed message array drew a
DeepSeek 400 on every attempt and the member was told to retry shortly. The
advice could not work, because the same array was rebuilt each time. Same family
as sirens-echo#449: a true sentence pointing somewhere useless.

The cause is `model_rejected` rather than `stage_failed`, so a malformed-request
class is countable on its own instead of collapsing into the stage.

## See also

- [tuning](sirens-echo-tuning.md) - where the constants live.
- [the budget](sirens-echo-budget.md) - the ceilings a turn spends against.
