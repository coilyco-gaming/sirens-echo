# Grounding action-claim corpus

`internal/community/groundingcorpus_test.go` holds both halves of the action-claim
property in one table, so a change to the detector is measured against replies it
must catch **and** replies it must not touch.

The property is narrow: *the reply asserts that a state-changing tracker action
has been completed, by this agent, in this turn.* Polarity, tense, and agency all
decide whether a sentence carries that assertion.

## Reading a row

| Column | Meaning |
| --- | --- |
| `reply` | The reply text, fed to `ValidateGrounding` with no supplied context |
| `rejectedNow` | What the validator does on `main` today |
| `shouldReject` | What it ought to do |
| `issue` | The issue that closes the gap, when the two disagree |

Tests assert `rejectedNow`, not `shouldReject`. CI therefore reports what ships
rather than what is wanted, and a row whose columns disagree is a tracked defect
rather than a red build.

## Retiring a row

When a fix lands, the assertion fails with a message naming the issue. Set
`rejectedNow` to the new behavior and clear `issue`. A row whose columns now
agree is a permanent guard and should keep its entry.

If a row changes in the wrong direction the message says `regression` instead,
because a guard that already agreed has broken.

## Why the false-positive half is the important one

A detector that misses a claim ships one bad reply. A detector that fires on a
correct reply fails the turn outright — `agent.go` routes a `ValidateGrounding`
error straight to `failTurn`, with no repair pass — so the member gets nothing.

The negations are the sharpest case. *"No issue has been filed for this"* is the
honest disclaimer the tracker is asking the agent to produce, and a pattern that
reads it as a claim makes the truthful answer unshippable. Any widening of the
detector has to keep those rows passing.

## Grounded claims

`TestGroundingAcceptsAClaimATrackerToolSupports` runs the rejected sentences
again with a tracker tool in the executed set. Without it, a detector could pass
the whole corpus by rejecting the verb rather than the ungrounded assertion.

## See also

See [the response contract](sirens-echo-prompt.md) and
[the deterministic battery](sirens-echo-battery.md), which states the rule this
corpus enforces — a check survives only when it cannot fire on a plausible
correct reply to its own case.
