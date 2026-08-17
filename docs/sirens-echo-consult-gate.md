# The consult gate

`consult` decides two things: how `cli-guard` dispatches an issue, and what a
human sees when they look for work that needs them. It was measured wrong in
both directions at once, which is why it has a document.

Written out it is **`autonomy/async-consult`**, one of an exclusive `autonomy/*`
set, so applying it removes `autonomy/headless`. This page says `consult` for
short; a command line has to say the whole thing or it silently applies nothing.

## What went wrong

The label is written once, when someone thinks to write it, and never again.
Nothing reconciles it with the thread afterwards. So it drifts two ways:

An answer arrives and the label stays. The work looks blocked, nobody takes it,
and the person who filed the question does not come back to it because the
board tells them it is still waiting. One measured instance sat four hours after
being answered, with its author working other issues the whole time.

A question is asked in prose and no label is written. The work looks free, so
nobody answers it. This is the worse direction: unlabelled is correctly fail
closed for dispatch and invisible to the human, so the issue is both undispatched
and unqueued.

Measured on one sweep: the queue advertised eighteen items when eight were real,
while hiding five. A reader who trusted it would have been wrong about fifteen
of twenty-three.

## The two habits

**Recording a decision removes the label.** You are already writing to the issue
at that moment, so this costs one call and nothing else.

**Asking a human a question adds it.** Same argument. A comment that ends with a
question for a director or an operator is a `consult` issue by definition.

Neither needs tooling. Both are one line attached to a write that is already
happening.

## Why a habit and not a check

A check would need to decide whether a comment contains an unanswered question,
which is a judgement rather than a pattern. The two directions are easy to
detect approximately and hard to detect correctly, and a hygiene check that is
wrong sometimes trains people to ignore it.

A sweep that *flags rather than enforces* is a reasonable middle: issues holding
`consult` whose thread contains a later decision, and issues whose recent
comments end in a question with no label. That query has been run by hand and
produced both lists, so it is known to work. It is worth building when someone
wants it and is not a prerequisite for the habits.

## See also

- [the decision index](https://forgejo.coilysiren.me/coilyco-gaming/sirens-echo/issues/315)
  - the human-readable version of the same queue.
