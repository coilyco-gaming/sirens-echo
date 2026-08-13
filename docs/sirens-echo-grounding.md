# Grounding a filing claim

The grounding check rejects an action claim the runtime did not perform, but its
first-person matcher reads only `I filed` and its siblings. The neutral profile
forbids all first-person voice, so for a neutral definition the two contracts
never overlap, and `A correction has been filed` passed with no tool call behind
it.

The check also reads the passive form, and three gates keep it off correct
replies. It anchors on a tracker artifact noun, so ordinary prose about the game
world is out of scope. It reads present perfect only, `has been filed` rather
than `was created`, because a simple past asserts a definite past time and is
therefore history rather than a claim about this turn. It runs per sentence and
skips any sentence that denies, hedges, supposes, asks, or credits someone else.

Polarity is the one that matters most. `No issue has been filed for this` is the
honest form of what this check exists to encourage, and an earlier version
rejected it, which was strictly worse than not checking at all. A grounding
error fails the turn with no repair loop, so a false positive costs a member
their whole answer while a false negative only lets a bad sentence through.
Under-firing is the correct direction here.

Reading present perfect only gives up some real claims, such as `A tracking
issue was created`. That is a deliberate trade rather than an oversight, and
`internal/community/groundingcorpus_test.go` pins both halves so any future
widening has to prove it does not regress the correct replies.

A passive claim counts as supported when the turn reached the tracker at all,
read or write. Demanding the exact write tool would reject a correct report of
an issue the runtime only looked up. Links are masked first, so the word
`issues` inside a URL path cannot seed a claim.

## Voices this still misses

Two shapes assert a completed filing and are deliberately not caught.

`A tracking issue was created` is a simple past passive. Catching it means
reading simple past as a claim, which is exactly what was producing false
positives on `created in June` and `closed last week`. Recovering it needs a way
to separate history from a self-claim that is not tense, and no cheap one exists.

`Sirens Echo has filed a correction` names the service as the actor. That is
unambiguous to a reader but not to the checker, because it cannot tell the
service's own name from a member's. `Octavian has filed a correction` has the
same shape and is a correct reply. Closing this one needs the configured
identity threaded into the check, which is a signature change across eighteen
call sites and worth doing deliberately rather than in passing.

The clipped form `Filed a correction for review` is caught. It requires the
sentence to open on the participle and name the artifact directly, so
`Filed issues are listed in the tracker` stays silent.

## See also

* [Knowledge gaps and corrections](sirens-echo-issues.md) - what a filing is for.
* [Grounding corpus](sirens-echo-grounding-corpus.md) - the pinned replies.
* [Prompt](sirens-echo-prompt.md) - the other response checks.
