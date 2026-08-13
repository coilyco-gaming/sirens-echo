# Content classes

`agent/content-classes.yaml` is the closed taxonomy a content classifier
chooses from. This file explains why it has the shape it has. The turn-path
wiring does not exist yet.

## Why allowed classes are listed

The requirement is a list covering every content type theoretically possible to
communicate. A denylist alone cannot satisfy that. Given only the categories to
refuse, an ordinary request has nowhere to land, so the model either forces it
into a deny bucket or answers outside the list. Both are worse than a wrong
answer, because both look like the classifier working.

So the allowed classes are enumerated too, and `other` is an explicit
catch-all. The loader refuses a taxonomy without one, and refuses one that
enumerates no allowed class, because either shape guarantees a wrong answer.

This is the closed-target-set rule from [the battery](sirens-echo-battery.md),
applied to a classifier instead of a check.

## Sensitivity is a refusal shape, not a verdict

`deny` decides whether the request is refused. `sensitive` decides how.

An ordinary block names its reason. A sensitive block emits a generic redirect
naming no category, because saying which rule fired tells the member exactly
what to avoid saying next time.

**Sensitive wins ties.** A request matching both resolves to the sensitive
class. A bedtime story trips creative long-form and minor suspicion together,
and naming the ordinary category out loud would leak the signal the sensitive
branch exists to hide. `Verdict` enforces this regardless of the order the
classifier returns.

## Two rules the file carries from elsewhere

A block is always visible. Silent no-reply was rejected so a refusal is never
mistakable for an outage.

A block is one sentence. Every volunteered justification is a handle the next
message can pull, so a refusal that argues its case is the longest thing the
service says.

## The distinction inside emotional-support

The class covers what a member *asks for*: comfort, reassurance, validation.

It does not cover the separate defect of the service asserting a member's inner
state. That is a grounding failure and no topic filter reaches it. The
distinction matters because a member asking whether their message read as
hostile is asking a question about text, which is ordinary community logistics
and must not be blocked. A filter that catches both is worse than no filter.

## What does not exist yet

Where the classifier runs, the second model call, the opt-in that applies this
to one profile and not another, and the span emission. A classifier that is
itself a model call inherits the timeout and backend-failure modes of the turn
it guards, so a cheap deterministic prefilter is worth considering before the
inference.
