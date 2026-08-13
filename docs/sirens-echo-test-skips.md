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

## The same shape elsewhere, now measured

The sweep that found the skips was Go tests only, so the neighbouring surfaces
were checked afterwards for the same shape, a success that means nothing ran.

- **Pack loaders** refuse an empty pack. The evaluation, board, and rate
  loaders each fail on zero cases, and the fixture pack fails on zero tools, so
  none can report a green load having checked nothing.
- **Shell scripts** all set `-euo pipefail` except `ci-docker-probe.sh`, which
  documents that it never fails because the caller wants the report.
- **`policy-check` named its inputs by hand.** Every tracked pack was listed at
  the time of checking, so nothing was unverified, but a pack added later would
  have been verified by nothing while the output stayed green. That one is now
  guarded: a file in `agent/` that no verify call names fails the suite.

The last one is the same defect as a silent skip, arriving from the other
direction. A skip stops running a check that exists. An unlisted pack never
gets a check at all, and both print success.

## See also

* [The battery](sirens-echo-battery.md) - the other place a check that cannot
  fail is worse than no check.
