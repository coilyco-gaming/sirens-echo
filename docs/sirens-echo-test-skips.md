# Every skip is a reviewed line

A skipped test and a passing test print the same word and exit the same way.
`ward exec test` reports `ok` for both, and nothing counts how many tests ran,
so a guard can stop running for months without anyone learning.

## What went quiet

Two guards were found unguarded at once. Deleting the thing each protected left
the suite green:

| test | why it skipped |
| --- | --- |
| `TestGraphPatternsNeverReachDeniedSources` | `AOS_CATALOG` is set nowhere in CI |
| `TestScratchRefusesSymlinkEscape` | planted a symlink at a partition path that stopped existing when the name became a hash, then reported symlinks unavailable |

The second is the sharper one. It was disabled by a **correct, unrelated fix**.
Every other stale reference to the old partition name was updated. Only the one
wrapped in a skip swallowed its error rather than failing.

## The check runs on what fires, not on what is written

A source scan for `t.Skip` would not have caught either. Both tests carried
their skip from the day they were written, and nothing in the source changed
when they went quiet. What changed was the skip starting to fire.

So `ward exec test-skips` runs the suite verbose, collects the tests that
actually emitted a skip, and compares that set to `.ward/test-skips.allow`.

## It fails in both directions

An **unreviewed skip** fails, which is the obvious half: a guard stopped running
and nobody decided that.

A **stale allowlist entry** fails too, and that half is not optional. An entry
that no longer fires reads as a known exception forever, and nobody deletes a
line that is not hurting. That is the same defect as the first, pointed the
other way, and it is how the list rots into decoration.

## Why an allowlist rather than a count

A count is a number someone bumps. A name is a line someone has to write a
reason next to, and the reason is the part a reviewer can disagree with.

## Not measured

Only Go tests. Whether the evaluation packs, `policy-check`, or the shell
scripts have an equivalent quiet-success path is an open question rather than a
cleared one.

## See also

* [The battery](sirens-echo-battery.md) - the other place a check that cannot
  fail is worse than no check.
