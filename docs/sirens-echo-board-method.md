# Sirens Deep board method

How the board at `agents/deep/packs/board.yaml` is built and scored. See
[the board](sirens-echo-board.md) for how to run it.

## Clauses come from the rendered prompt

A clause is an obligation the rendered prompt actually states, cited by line
against `agents/deep/rendered/prompt.txt`. `ward exec prompt-check` fails
when that snapshot drifts from its sources, so a doctrine edit surfaces as a
board whose citations no longer match.

Deep has no roles, personalities, or adjacency, so the roster axis that derives
the sibling agent-compose board has no analogue. The prompt is the axis instead.

## Pairs, and why the in half is never optional

Every clause is paired. The in half is where the clause requires Deep to act.
The out half is where it requires Deep to decline.

**The pair is the scoring unit, not the case.** A clause that passes one half
and fails the other is a clause failure, not fifty percent.

The in half is a negative control. Six of the eight clauses on the full board
are refusals, so a Deep that refused everything would score near-perfect on out
halves alone. `LoadBoardPack` rejects a pair holding one half for that reason,
and no filtering step may drop an in half for being easy. In the sibling suite
the only real boundary failure on the first graded board was an in-half failure
that the earlier filter would have deleted before a human saw it.

## What belongs here rather than in the battery

The board holds only what a human has to decide. Anything a scoped or anchored
check can decide belongs in the deterministic battery, which is cheaper, runs
on every deployment, and does not consume grading time. See
[the battery](sirens-echo-battery.md) for its check types and authoring rule.

`pronoun-defaults` used to be a board pair and is now a battery case, because
`pronoun_policy` scores the pronoun used for a named subject directly. Keeping
a graded copy alongside it would be two guards over one behavior, which is the
drift this split exists to avoid.

The reverse move is also expected. A battery check that turns out to collide
with a correct reply is deleted rather than tuned, and the behavior it was
reaching for becomes a board pair.

## There is no mechanical scorer on the board

Not narrowed, not tuned, not advisory-with-a-threshold. The board records what
the deployed validators say in a `structural` field and treats it as evidence.

This is measured rather than preferred. The sibling suite graded the same
responses by hand after running a regex discriminator tier. Across nine cases
the two agreed on nothing that mattered: one false positive, two false
negatives, and six agreements where nothing happened. It was deleted, not tuned.

## Current scope

The board ships as a two-clause pilot: `no-invented-surface` and
`trusted-principal`. The first has a documented real-world regression behind
it, which gives the board a validity check the sibling suite never had. If its
out half does not reproduce issue 88 against the pre-fix bundle, it is not
measuring.

The remaining five clauses wait on the first graded result. In the sibling
suite the generator's predictions about which cases would discriminate ran one
for three, so authoring the rest against conventions validated only by the
generator's own judgment is the mistake that record already documents.

Two open items:

- No case requires a tool. The `tool-grounding` pair waits on the deployed
  guardfile's tool list, which is the thing this repo cannot read. Roster
  breadth was never the blocker and a count written here would age.
- `looked` is a real gap in the `ValidateGrounding` verb list. Closing it makes
  a live service stricter, so it is recorded rather than changed here.
