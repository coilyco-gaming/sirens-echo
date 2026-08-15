# Sirens Deep evaluation board

Deep answers strangers in a guild the operator does not moderate, so what it
refuses matters as much as what it answers. That is measured across two layers
which never share a file.

## Two layers

**Layer 1 is the deterministic battery.** `agents/deep/packs/evaluation.yaml`, run
with `ward exec eval-deep`. It hard-fails and needs no human. Scoped and
anchored checks decide pronouns, prompt leakage, injection canaries, and
operator-identifier echo, on top of the structural validators the deployed path
already runs. See [the battery](sirens-echo-battery.md).

**Layer 2 is the human-graded board.** `agents/deep/packs/board.yaml`, run with
`ward exec board-deep`. It gates nothing. The runner emits a dataset and
reports no verdict, so a non-zero exit means the run did not happen rather
than that Deep failed.

The split exists because judgment and gating want opposite things. A gate has
to be mechanical and cheap enough to run on every deployment. Judgment has to
be able to say "this reply is technically compliant and still wrong", which no
pattern can say.

## The triple

| Seat | Who | Note |
| --- | --- | --- |
| Generator | AI Engineer seat, interactive with Kai | Cannot grade what it authored |
| Subject | `deepseek-v4-flash` on `sirens-echo/deepseek` | The deployed route |
| Grader | Kai | Ground truth, not validated against a rubric |

No party occupies two seats. Deep is a convenient case because the board grades
the model that actually ships, so there is no declared-versus-executed tier
drift to correct for.

## Running it

```sh
ward exec board-deep > agents/deep/evaluations/<date>-<seat>.yaml
```

Needs `AGENT_PROXY_URL`, `AGENT_PROXY_MODEL` naming the profile's route, and
`OTEL_EXPORTER_OTLP_ENDPOINT`. Supply `SIRENS_ECHO_MCP_ROSTER` when a case
requires a tool. Without one the roster is empty and a tool case fails for a
reason that is not the agent's.

`SIRENS_ECHO_BOARD_EPOCHS` defaults to 5. The grader reads epoch 1 of each
record against its target and scores pass or fail, noting only deductions. The
remaining epochs stay in the dataset as a failure-spread estimate at no grading
cost, which answers the single-run gap without consuming human time.

Anchor a deduction to a verbatim span from the response, so a critique is
auditable rather than impressionistic.

A dataset is evidence. Keep it under that agent's `evaluations/` by date and seat. A retired
result is archived rather than deleted, because the before-and-after is the
argument that a doctrine change worked.

## See also

- [Board method](sirens-echo-board-method.md) - clauses, pairs, and scoring.
- [Battery](sirens-echo-battery.md) - the deterministic layer.
- [Response profiles](response-profiles.md) - what each profile promises.
- [Prompt composition](sirens-echo-prompt.md) - how the snapshot is produced.
