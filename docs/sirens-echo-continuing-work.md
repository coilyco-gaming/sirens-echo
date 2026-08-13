# Work continuing past the turn

`Sirens Echo will keep watching the server` is not a claim about the past, so
none of the grounding gates read it. No tool call can ground it either. A turn
ends when the reply is sent and this runtime holds no scheduler, so the promise
names something no code will ever do.

The member-visible outcome is someone waiting for a message that is never
coming, which reads as being ignored rather than as a failure.

## One definition, two readers

The pattern is the `no-continuing-work-claim` case in `agent/evaluation.yaml`,
shared with the runtime rather than copied into it.

`TestContinuingWorkClaimIsPinnedToTheDeploymentGate` loads the pack and asserts
the two are character-identical, so editing either alone fails the suite. Two
copies of one definition drift quietly, and the failure mode is asymmetric: the
gate keeps passing while the runtime stops matching, or the reverse, and
whichever direction nobody is watching is the one that costs a member an answer.

Before this check the gate held the definition alone, so the shape failed a
build and shipped at runtime.

## What it does not fire on

It requires a named subject followed by `is now` or `will`, so a correct refusal
such as `Sirens Echo will not keep watching the server` does not match, and
neither does prose about the game world. A grounding error fails the turn with
no repair loop, so under-firing is the correct direction.

**The verb list is the second bound and it was too narrow.** It was built from
the promise-to-keep-watching defect, so a lookup escaped: `Sirens Echo is now
searching the tracker` carried the right subject and tense and no matching verb.
`search`, `look up`, `query`, `retrieve` and `fetch` were added for
[sirens-echo#341](https://forgejo.coilysiren.me/coilyco-gaming/sirens-echo/issues/341),
measured at zero false positives across twelve correct replies.

**The subjectless form still escapes.** `Searching the issue tracker for an open
ticket` is what a live run actually produced, and requiring the subject is what
keeps `The Eco app is now tracking prices` clean. A corpus row holds it open.

It is English-only, inherited from the gate's pattern rather than chosen here.
Tracked with the rest of the language coverage in issue 253.

## See also

- [Grounding a filing claim](sirens-echo-grounding.md) - the past-tense gates.
- [Grounding corpus](sirens-echo-grounding-corpus.md) - the pinned replies.
