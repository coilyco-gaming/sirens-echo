# Evaluation

Deep answers strangers in a guild the operator does not moderate, **so what it refuses matters as much
as what it answers**. This page is how that is measured: the two layers, the triple, the pairing rule,
and the shared `aos-eval` this deployment grades through.

## Two layers that never share a file

**Layer 1 is the deterministic battery**, `agents/deep/packs/evaluation.yaml`, run with `just eval-deep`.
It hard-fails and needs no human. **Layer 2 is the human-graded board**, `agents/deep/packs/board.yaml`,
run with `just board-deep`, which **gates nothing**: it emits a dataset and reports no verdict, **so a
non-zero exit means the run did not happen rather than that Deep failed**.

The split exists because **judgment and gating want opposite things**: a gate must be mechanical and
cheap enough for every deployment, while judgment has to be able to say "this reply is technically
compliant and still wrong", **which no pattern can say**.

**The board holds only what a human has to decide.** Anything a scoped or anchored check can decide
belongs in the battery, **a graded copy alongside a mechanical one being two guards over one behavior**.
The reverse move is expected too, and a battery check that fires on a correct reply is **deleted rather
than tuned**. That pack's header carries its own rule, its assumed validators, and each live check kind,
and it is the current list where this page is a summary.

**Nothing mechanical scores the board.** It records what the deployed validators say in a `structural`
field and treats that as evidence rather than a verdict. The battery is not the same claim: its checks
have closed target sets rather than phrase lists, and the pack header carries that argument.

## The triple

Three seats, **and no party occupies two of them**: a **generator**, a frontier model at high effort
interactive with Kai, which **cannot grade what it authored**; a **subject**, the commodity tier pinned
to `deepseek-v4-flash` on the deployed route; and a **grader**, Kai, ground truth rather than something
validated against a rubric. **A weaker subject removes the ceiling effect that makes a gate certify
rather than measure**, and a human grader removes self-judging and the unvalidated-judge problem at once.

`SIRENS_ECHO_BOARD_EPOCHS` defaults to 5. The grader reads epoch 1 and scores pass or fail, **and the
rest stay in the dataset as a failure-spread estimate at no grading cost**.

## The pair is the scoring unit

**Every clause is paired**, the in half requiring Deep to act and the out half requiring it to decline,
**and the pair is the scoring unit, not the case**. **The in half is a negative control**: most clauses
on this board are refusals, **so a Deep that refused everything would score near-perfect on out halves
alone**, and `LoadBoardPack` rejects a pair holding one half.

**A clause is an obligation the rendered prompt actually states**, cited by line against the tracked
snapshot. `just prompt-check` fails when that snapshot drifts, and
`TestBoardClauseCitationsStillPointAtTheirClause` fails when a citation no longer lands on its clause.
Deep has no roles, personalities, or adjacency, **so the prompt is the axis** where agent-compose uses
its roster.

The pilot slice is five clauses, ten cases. `no-invented-surface` reproduces issue 88, **which gives the
board a validity check the sibling suite never had**: if its out half does not reproduce that against
the pre-fix bundle, **the board is not measuring**. The rest wait on the first graded result, because
the generator's predictions about which cases discriminate are not yet a measured quantity.

## Grading runs through aos-eval

[`aos-eval`](https://forgejo.coilysiren.me/coilyco-flight-deck/agentic-os) is the shared grading half,
released from agentic-os on its own `aos-eval-v*` train: the record schema, the pairing rule, human
annotation, the failure taxonomy, and a one-way display export. It holds no runner and no model client,
**so grading spends no tokens and touches nothing deployed**.

`board-deep` emits `dataset:` where each record is aos-eval's `Sample` plus its `output`, so
`aos-eval annotate` reads the file with no adapter. `role` carries the grouping axis, which here is the
clause, so `--role earn-the-reply` grades one pair. The epochs ride along in `responses`, which aos-eval
ignores. `eval/aos-eval-profile.yaml` declares this deployment's one column, its label set, its 50-word
critique cap, and the fields a boundary case cannot omit.
`TestBoardDatasetCarriesTheAosEvalSampleShape` pins the contract, **because a rename on either side
would break grading silently**.

```bash
just board-deep > agents/deep/evaluations/<date>-<seat>.yaml
just grade-check agents/deep/evaluations/<date>-<seat>.yaml
just grade agents/deep/evaluations/<date>-<seat>.yaml
just taxonomy <dataset> <annotations>
```

`board-deep` needs `AGENT_PROXY_URL`, `AGENT_PROXY_MODEL`, `OTEL_EXPORTER_OTLP_ENDPOINT`, and
`SIRENS_ECHO_MCP_ROSTER` when a case requires a tool, **because without one a tool case fails for a
reason that is not the agent's**. `AOS_EVAL_REF` pins a tag when a run must be reproducible.

**Anchor a deduction to a verbatim span**, which aos-eval checks against the output rather than taking on
trust, and treat **a dataset as evidence**: keep it by date and seat and archive rather than delete a
retired one, **because the before-and-after is the argument that a doctrine change worked**.

## The boundary declaration

`eval/boundaries.yaml` declares every boundary this deployment holds, once, in aos-eval's declaration
shape. `aos-eval boundaries derive` reads it as-is, and `aos-eval boundaries check` compares the slots it
derives to what the board actually authored.

**That coverage number is meant to read low.** The declaration is the deployment's boundary inventory
while the board is authored against prompt clauses, so the two taxonomies do not yet meet, and a number
that counted the board as complete would be certifying rather than measuring. It names every missing case
and stays wrong out loud until those cases exist. Deriving the board from the declaration is tracked by
#846. **Nothing here names a bot**, identity being a deployment concern.

`just boundaries` prints the paired case list, and `just boundaries-check` fails when a declared boundary
no longer resolves against the source it names. That source-drift check stays local because `aos-eval
boundaries check` compares slots to a dataset instead, so the two answer different questions.

## The rest of the stack

* [aos-eval](https://forgejo.coilysiren.me/coilyco-flight-deck/agentic-os/src/branch/main/docs/aos-eval.md) -
  the shared layer this grades through, plus the probe layer under it.
* [agent-compose evaluation](https://forgejo.coilysiren.me/coilyco-flight-deck/agent-compose/src/branch/main/docs/evaluation.md) -
  the other consumer: same pairing rule, a composed prompt rather than a live harness.
* [Deleting the Mechanical Scorer](https://coilysiren.me/posts/deleting-the-mechanical-scorer/) - why both
  boards are hand-graded and how the shared layer was extracted.
