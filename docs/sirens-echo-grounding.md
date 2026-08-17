# Grounding

Four rules, and the corpus that keeps them from firing on a correct reply.

## Grounding a filing claim

The grounding check rejects an action claim the runtime did not perform, but its first-person matcher
reads only `I filed` and its siblings, **and the neutral profile forbids all first-person voice**, so
for a neutral definition the two contracts never overlap and `A correction has been filed` passed with
no tool call behind it. The check also reads the passive form, with three gates keeping it off correct
replies: it anchors on a tracker artifact noun, **so ordinary prose about the game world is out of
scope**; it reads present perfect only, `has been filed` rather than `was created`, **because a simple
past asserts a definite past time and is therefore history rather than a claim about this turn**; and it
runs per sentence, skipping any sentence that denies, hedges, supposes, asks, or credits someone else.

**Polarity matters most.** `No issue has been filed for this` is the honest form of what this check
exists to encourage, and an earlier version rejected it, **which was strictly worse than not checking at
all**. A grounding error fails the turn with no repair loop, **so a false positive costs a member their
whole answer while a false negative only lets a bad sentence through**: under-firing is the correct
direction. Reading present perfect only gives up some real claims, **a deliberate trade** that
`groundingcorpus_test.go` pins both halves of, so any future widening has to prove it does not regress
the correct replies. **A passive claim counts as supported when the turn reached the tracker at all,
read or write**, because demanding the exact write tool would reject a correct report of an issue the
runtime only looked up, and links are masked first so the word `issues` inside a URL path cannot seed a
claim.

**One shape asserts a completed filing and is deliberately not caught.** `A tracking issue was created`
is a simple past passive, and catching it means reading simple past as a claim, **which is exactly what
was producing false positives on `created in June` and `closed last week`**. The clipped form `Filed a
correction for review` **is** caught, requiring the sentence to open on the participle and name the
artifact directly, so `Filed issues are listed in the tracker` stays silent.

`ValidateGrounding` is one function and **four independent rules**, so a refusal reports the rule rather
than the family: `grounding.invented_channel`, `grounding.claimed_action`, `grounding.tracker_action`,
and `grounding.continuing_work`. **Knowing only that grounding refused left all four to re-run by hand
against the reply.**

## Grounding a channel name

The check rejects a `#channel` the reply invented, and its allowlist was built from supplied context
alone, **a source near enough to empty for any channel a turn actually reached**. `restrict` cannot glob
Discord snowflakes and the snowflakes share no prefix, so the deploy guardfiles fix each channel into
its own tool, **and the consequence is that a channel's name lives in the tool name,
`list_general-message`, and in the tool result, and never in supplied context**. A turn that read
`#general` and said so was refused for inventing it, and Echo has the same shape with her `eco-`
channels, **so this is structural rather than one lane's problem**.

The allowlist now also reads **the completed tool names**. The two available sources are not equivalent
and only one is safe to widen a hallucination guard with: **a tool name is deploy-authored, reviewed,
and fixed at image build**, so it adds no input anyone outside review controls, while **a tool result is
community text**, so a member posting `#nonexistent` would enter it into the allowlist and teach the
check to accept a channel nobody has. Outcome is deliberately not consulted, because **a failed call
still proves the channel exists** and a reply reporting that failure names it correctly.

**The check refuses a whole message, so one channel mention discards every other paragraph.** A roster
summary naming `#general` in one line lost eleven correct ones, two of them live outage findings that
reached nobody, and Sirens Deep runs a single execution slot, **so the 51 seconds it burned also timed
out the member queued behind it**. That is why under-firing is the right direction here too.

## The action-claim corpus

`internal/community/groundingcorpus_test.go` holds both halves of the property in one table, **so a
change to the detector is measured against replies it must catch and replies it must not touch**. The
property is narrow: *the reply asserts that a state-changing tracker action has been completed, by this
agent, in this turn*, with polarity, tense, and agency all deciding whether a sentence carries that
assertion.

Each row carries the reply text, `rejectedNow` (what the validator does on `main` today),
`shouldReject` (what it ought to do), and the issue that closes the gap when the two disagree.
**Tests assert `rejectedNow`, not `shouldReject`**, so CI reports what ships rather than what is wanted
and **a row whose columns disagree is a tracked defect rather than a red build**. When a fix lands the
assertion fails with a message naming the issue: set `rejectedNow` to the new behavior and clear
`issue`, and a row whose columns now agree is a permanent guard that keeps its entry. A row changing in
the wrong direction says `regression` instead, **because a guard that already agreed has broken**.

**The false-positive half is the important one.** A detector that misses a claim ships one bad reply; a
detector that fires on a correct reply fails the turn outright, with no repair pass, **so the member
gets nothing**. The negations are the sharpest case: `No issue has been filed for this` is the honest
disclaimer the tracker is asking the agent to produce, **and a pattern that reads it as a claim makes
the truthful answer unshippable**. `TestGroundingAcceptsAClaimATrackerToolSupports` runs the rejected
sentences again with a tracker tool in the executed set, **because without it a detector could pass the
whole corpus by rejecting the verb rather than the ungrounded assertion**.

## Counting a behaviour in evidence already paid for

`just evidence-scan` counts tool-call markup across every committed run record. **The behaviour runs at
roughly half a percent per attempt**, so a rate case at the usual fifteen runs has an expected count of
0.08: it reads zero and establishes nothing, **and worse, it would read as fixed once the reply path
refuses markup, when it was never measurable at that size**. The datasets already hold hundreds of
attempts. The scanner calls `ValidateNoToolCallMarkup`, reading the same patterns the deployment gate
and the reply path read, **because two copies of one definition drift in whichever direction nobody is
watching**.

**Two limits belong with any number this produces.** A dataset that parses to no replies is a parse that
found nothing rather than a run that produced nothing, so the scanner exits non-zero when no structured
dataset parsed at all, **because a confident zero over an empty read is the quietest possible wrong
answer**. And the pattern set covers only the delimiter syntaxes that have been measured, **so a zero
for a model family whose syntax was never observed says nothing about that family**.
