# The rate pack

`agent/rate-echo.yaml` and `agent/rate-deep.yaml`, run with `ward exec
rate-echo` and `ward exec rate-deep`. They measure how often an intermittent
behavior happens, gate no deployment, and are never wired into CI. See [the
battery](sirens-echo-battery.md) for the gate and [the
board](sirens-echo-board.md) for the graded layer.

## Why a third instrument

Neither existing one can hold an intermittent behavior. The battery hard-fails
a deployment, so a case that fails 13 percent of the time makes the gate flaky.
The board is human-graded, and grading is per-artifact attention, so it cannot
economically run one case fifteen times.

Before this existed, an observed rate lived only as prose in an issue body.
Nothing regenerated it, or noticed a fix taking 13 percent to 4 rather than 0.

## How a run is scored

A case declares its own `runs` and `max_failure_rate`. Each attempt is scored by
`community.ScoreEvaluationCase`, the same function the gate uses, so the two
instruments cannot drift. A rate for a check the gate does not apply would
measure something nobody enforces.

An attempt passes, fails, or errors. An error is a failure of the substrate
rather than of the agent, such as a 502 from Agent Proxy, and is excluded from
the denominator. Counting one as a behavioral failure corrupts the rate.

The run exits non-zero when a case beats its declared ceiling, when every
attempt of a case errored, or when the boundary median is not below the
conversational one. An unmeasured case or comparison is not a passing one.

## The dataset is the evidence

Every reply is persisted verbatim. Three first-pass findings in the QA that
motivated this pack were defects in the check rather than the agent, and only
reading the text separated them. Provenance travels with it: a rate without its
definition, pack, model, and roster is not comparable to the next run.

Every failing check is recorded rather than the first. A run recorded a user ID
disclosure as a handle echo and read zero ID leaks.

## The promotion path

A case starts here to establish its rate. When a fix drives that rate to zero
and holds at high N, the case may move into `agent/evaluation-deep.yaml` as a
deterministic regression.

Do not promote on a small clean run. Five runs put a weak upper bound on the
true rate: a behavior at 13 percent passes 5 of 5 about half the time.

Promotion is also where the battery's rules reattach. A promoted case must not
be able to fire on a correct reply, and its target set must be closed.

## Cost

One attempt is one completion plus up to six tool rounds. Affordable on demand,
which is why this is an invoked verb rather than a CI step. Field-by-field
provenance: [rate provenance](sirens-echo-rate-provenance.md) and [errors](sirens-echo-rate-errors.md).

## What it cannot measure

**Whether a tool told the truth.** Checks score the reply, and
`ValidateGrounding` scores it against the tools that ran. Nothing scores a tool
result against the world, so a tool returning zero for a valid question yields
a faithful, grounded, confidently false answer that every instrument here
passes. Observed: an item filter matching an internal key rather than the name
a member types reported zero offers for a market holding 913 of them.

A fixture cannot close it. A fixture declares its own result, so it tests how
the model handles a payload, never whether the payload was right.

**Whether a number describes deployed Deep.** A composed definition reads a
placeholder where the pod injects the real agent-compose bundle, so the
instructions differ and not only the build. Datasets record it as `composed`.

A behavior with no deterministic check. Paraphrase disclosure is the standing
example: the reply discloses while quoting nothing, so no expression separates
it from a correct answer. That belongs on the board.
