# The boundaries declaration

[`eval/boundaries.yaml`](../eval/boundaries.yaml) declares every boundary this
deployment holds, once. The evaluation board derives from it rather than being
maintained beside it, so adding a boundary moves the case list on its own. The
board's method is [the evaluation board](sirens-echo-eval-board.md).

## Nothing here names a bot

Identity is a deployment concern (aos#778), and naming a bot would violate
#836's acceptance test. It is also a feasibility constraint. Paired, these
boundaries are already about as many cases as a human grades in one sitting,
and a bot dimension would multiply that.

## Fields

* `id` - stable, and what a case is reported under.
* `origin` - where the boundary actually lives, as `path` or `path#fragment`.
* `derived` - whether the entry was read out of that source.
* `rule` - the boundary in one sentence.
* `inside` - the arm where the agent must act.
* `outside` - the arm where it must decline, or must not fire.
* `seed` - an existing breaching record and its issue, where #846 absorbs one.

## The pair is the scoring unit

Every boundary produces exactly two cases and they score together. Without the
outside arm a degenerate always-decline policy scores perfect conformance,
which is the defect the reference board added pairing to catch. A boundary
missing either arm is a failure, not a partial entry, so
`just boundaries-check` rejects it.

## Derived against prose

`derived: true` means the fragment was read out of the named source and
`just boundaries-check` confirms it is still there. Those come from two
machine-readable places:

* `agent/content-classes.yaml` - the closed content taxonomy.
* `internal/community/turnstages.go` - the reply-check constants, with
  grounding expanded to its four distinct refusal reasons.

`derived: false` means the boundary is prose in a policy skill and the entry
was written by hand. **The checker cannot tell when one of those clauses
moves**, so it reports the count as undriftable rather than passing silently.
Closing that gap means giving the prose clauses a declaration, and it is the
reason the board is not fully derived on day one.

## Commands

```
just boundaries         # print the paired case list
just boundaries-check   # fail when a declaration no longer resolves
```

`boundaries-check` verifies that every `origin` path exists, that every
`derived` fragment is still present in it, that no `id` repeats, and that no
boundary is missing an arm. It does not check wording, and it does not run a
model.

## What it does not do

It holds no questions and no grades. A generator seat reads the pairs and
writes the questions, the commodity subject answers them, and a human grades.
Keeping the questions out of this file is what lets the declaration stay the
thing that derives, rather than becoming a second case list to maintain.

## Related

* [The evaluation board](sirens-echo-eval-board.md) - the method and the triple.
* [The rate pack](sirens-echo-rate.md) - the instrument the board absorbs.
