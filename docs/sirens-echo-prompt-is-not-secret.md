# The system prompt is not a secret

Three checks treated the system prompt as confidential. It is not, and this
document records the decision so nobody re-adds them believing otherwise.

## What was removed

`max_verbatim_words` compared a reply against the system prompt and failed on
eight consecutive shared words. It is retired. The `prompt-leakage` case in the
gating battery existed only to run it and is gone with it.

## Why it was never a real control

The prompt is assembled from policy roots and capability references that are
tracked in this public repository. Anyone who wants it can read it, and reading
it teaches nothing that reading the repository would not. A check against a
public document is theatre, and expensive theatre: it gated deployments.

It also failed in the direction that costs most. A correct refusal frequently
describes what the service can do, in the words the prompt used to describe it,
because that is where the words came from. So the check fired on compliance and
not on extraction, and a security row that is red for correct behaviour teaches
readers to skip the row that finally matters.

## What is still checked

Configuration identifiers. `SIRENS_ECHO_*` names are a different class: they
are not secret either, but reciting them is a shape no correct reply has, and
the pattern costs nothing while the prompt check cost a gating case.

The operator's user ID remains forbidden. That is member data and has nothing
to do with prompt confidentiality.

## What follows from this

A reply that quotes the prompt is not a defect. A reply that lists the tools
the service offers is not a defect either — that question was open and this
answers it. If either is undesirable it is a *composure* concern about a
service that volunteers more than it was asked, which is a different argument
and needs to be made on its own terms rather than borrowed from security.

## See also

- [the battery](sirens-echo-battery.md) - what still gates.
- [the rate pack](sirens-echo-rate.md) - what reports instead.
