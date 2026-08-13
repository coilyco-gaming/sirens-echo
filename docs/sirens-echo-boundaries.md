# Declined requests, and why this is prose

`agent/content-classes.yaml` names six denied classes. Nothing reads it. The
classifier that would enforce them is still a design decision, so until it
lands the only thing that can carry these rules is the policy root the model
reads. That is what `references/boundaries.md` is.

## What was actually enforcing them before

Nothing in this repository. Two searches settle it: `ContentClassifierPrompt`
and `Verdict` are defined and never called outside tests, `LoadContentTaxonomy`
is called only by `policy-check`, and no class id appears in either rendered
prompt.

The base model's own alignment was the whole control. That is real, and it is
also not deployment-specific, not measurable from here, and not something this
repository can claim. For explicit content it is probably adequate. For a
suspected minor it is not, because the requirement is a particular refusal
shape rather than a decline.

## The refusal shape is part of the rule

A sensitive decline names no category and gives no reason. Saying which rule was
hit tells the next person exactly what to avoid saying, which defeats the rule.
That is why `ContentClass.Sensitive` changes the refusal shape rather than the
verdict, and the prose has to carry the same distinction or the two disagree the
day the classifier lands.

An ordinary decline may name itself, because a member who can see the boundary
can work with it instead of spending another message discovering it.

## Where each rule lives

Five classes live in `references/boundaries.md`. Emotional support lives in the
response policy instead, beside the wider prohibition on entering emotional
territory at all, because that rule is about more than declining a request. It
is deliberately not repeated, since two copies of one rule drift apart.

## This is not a filter

Nothing inspects a request before the model sees it. Every line in that file is
a rule the model may read and still break, which is the same weakness as every
other prose rule in this repository and the reason the classifier issue stays
open. The gap worth acting on was between no control and a weak one, not
between a weak control and a good one.

Cost: Echo's prompt grows about 1500 bytes, paid on every turn forever. That is
the largest single increase tonight and it buys the only statement of these
boundaries that exists.

## What replaces this

When the classifier lands, these rules move behind it and this file shrinks to
whatever the model still needs to know about refusal shape. Nothing here
forecloses where the classifier runs, whether it is a second model call, or
whether it is opt-in.
