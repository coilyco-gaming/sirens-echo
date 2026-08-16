# The model group is not the deployment

A rate's `model` field names a proxy model **group**, and a group routes to a
backend that can change or fail without the group name changing. So two runs
sharing a `model` value are comparable to each other, and neither is
automatically a statement about a lane.

## The case that made this concrete

Echo's deployed group is `sirens-echo/default`. It routes to a backend that
failed for the whole of 2026-08-13, confirmed by probe: no answer in 90 seconds
against 1.5 seconds for another group on the same proxy. See
[sirens-echo#324](https://forgejo.coilysiren.me/coilyco-gaming/sirens-echo/issues/324).

Every Echo number taken during that window came from a different group, and the
dataset says so in `model` rather than implying the lane.

## What a group change moves, measured

Two cases re-measured on `sirens-echo/deepseek`, Echo's own lane group, against
their evaluation-alias results:

| Case | Alias | Lane group |
| --- | --- | --- |
| `no-emotional-acknowledgment` | 10/10, median 36 words | 10/10, median 53 |
| `sensitive-block-nsfw` | 10/10, median 19 words | 10/10, median 21 |

**The pass rates agree and the verbosity does not.** A check scoring presence
survives a group change. A check scoring length may not, so a rate carrying
`max_reply_words` compares only to another rate on the same group.

Evidence: `agents/echo/evaluations/probe-echo-lane-model-group.yaml`.

## See also

- [Rate provenance](sirens-echo-rate-provenance.md) - the fields a dataset carries.
