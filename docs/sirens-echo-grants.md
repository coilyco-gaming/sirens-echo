# Per-requester authority

What a job may do is determined by who requested it, not by which pod runs it.

## The three questions, answered

**1. What does "acts as the principal" mean?** Filtered grants over one
credential. The pod keeps its own identity and the harness narrows which job
kinds a given principal may cause.

The alternative, per-principal credentials, is credential brokering, which is
adjacent to a deferred item and should not be entered by accident. The filtered
form is also honest about what happened: one identity performed the effect on
someone's behalf, and the record says whose behalf.

**2. Where does the grant table live?** In the access policy document, beside
admission.

The coupling concern is real and is the same one raised about per-channel
addressing. It is accepted here because a grant answers the same
deployment-owned question about the same principals, in the document a reviewer
already reads before changing who can reach the agent. Splitting it would mean
two documents that must agree about the same people.

If that turns out wrong, the table is a self-contained field and moving it is a
loader change.

**3. Default for an unlisted principal?** Deny. `Evaluate` already fails
closed on an unlisted guild, and authority should not be the one gate that fails
open.

## The model

```yaml
grants:
  principals:
    - id: "318190481467244544"
      note: the trusted principal
      kinds: all
    - id: "222190481467244544"
      note: a guild member
      kinds: ["echo"]
```

`kinds` takes `all` or an explicit list, the same `Allowlist` shape channels and
users already use. `all` is permitted and conspicuous in a diff.

A kind that is not declared in `JobKinds` fails validation, so a table cannot
grant something that does not exist, and a malformed table stops startup rather
than silently denying everyone.

## Denial is an outcome, not a silence

A refused submission still creates a job record, moved to `failed` with the
outcome `not permitted`. A denial is attributable: it appears in the principal's
listing, carries a reason, and can be asked about afterwards.

A denial that left no record would be the one event nobody could investigate,
which is the opposite of what an authority model is for.

The reason is stated to the caller. `GrantedKinds` also lists what a principal
holds, so they can be told rather than discovering it by being refused.

## No table grants everything

A deployment that declares no `grants` block is unchanged. That is correct
before a table is adopted and wrong after the guild widens, so adopting one is
part of opening the guild rather than a follow-up.

## What this is not

It does not add a second authorization checkpoint beyond admission, and it adds
no human-in-the-loop approval step. Those are items 7 and 8 and remain out of
scope. If a design starts needing either, that is a signal to stop and get them
approved rather than absorb them quietly.

It does not decide who may *reach* the agent: that is admission, and it runs
first and separately. See [attribution](sirens-echo-attribution.md), which
records who asked, and [access](sirens-echo-access.md), which decides who may.
