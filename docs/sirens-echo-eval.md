# Evaluation

Deep answers strangers in a guild the operator does not moderate, **so what it refuses matters as much
as what it answers**. This page is how that is measured: the two layers, the declaration they derive
from, and where the shared `aos-eval` grading layer fits.

## Two layers that never share a file

**Layer 1 is the deterministic battery**, `agents/deep/packs/evaluation.yaml`, run with `just
eval-deep`. It hard-fails and needs no human. **Layer 2 is the human-graded board**,
`agents/deep/packs/board.yaml`, run with `just board-deep`, which **gates nothing**: the runner emits a
dataset and reports no verdict, **so a non-zero exit means the run did not happen rather than that Deep
failed**.

The split exists because **judgment and gating want opposite things**. A gate must be mechanical and
cheap enough to run on every deployment, while judgment must be able to say "this reply is technically
compliant and still wrong", **which no pattern can say**.

**The board holds only what a human has to decide.** Anything a scoped or anchored check can decide
belongs in the battery: `pronoun-defaults` was a board pair and is now a battery case, **because a
graded copy alongside it would be two guards over one behavior**. The reverse move is expected too, and
a battery check that collides with a correct reply is **deleted rather than tuned**.

## The triple

Three seats, **and no party occupies two of them**: a **generator**, a frontier model at high effort
interactive with Kai, which **cannot grade what it authored**; a **subject**, the commodity tier pinned
to `deepseek-v4-flash` on the deployed route; and a **grader**, Kai, ground truth rather than something
validated against a rubric. **A weaker subject removes the ceiling effect that makes a gate certify
rather than measure**, and a human grader removes self-judging and the unvalidated-judge problem at
once.

`SIRENS_ECHO_BOARD_EPOCHS` defaults to 5. The grader reads epoch 1 of each record and scores pass or
fail, **and the remaining epochs stay in the dataset as a failure-spread estimate at no grading cost**.

## The pair is the scoring unit

**Every clause is paired**, the in half requiring Deep to act and the out half requiring it to decline,
**and the pair is the scoring unit, not the case**. **The in half is a negative control**: six of the
eight clauses on the full board are refusals, **so a Deep that refused everything would score
near-perfect on out halves alone**, and `LoadBoardPack` rejects a pair holding one half. **In the
sibling agent-compose suite the only real boundary failure on the first graded board was an in-half
failure that its earlier filter would have deleted before a human saw it.**

**A clause is an obligation the rendered prompt actually states**, cited by line against the tracked
snapshot, and `just prompt-check` fails when that snapshot drifts, **so a doctrine edit surfaces as a
board whose citations no longer match**. Deep has no roles, personalities, or adjacency, **so the prompt
is the axis** where agent-compose uses its roster.

## The board is derived from one declaration

**The reference board has three boundaries across its roles. This repository has tens**: 13 content
classes, 9 reply validators, and prose clauses across five policy skill roots, **so the case list cannot
be hand-maintained**. Boundaries are declared once in [`eval/boundaries.yaml`](../eval/boundaries.yaml)
and the board is **derived** from it, every boundary producing two cases. Format:
[boundaries](sirens-echo-boundaries.md).

`just boundaries` prints the paired case list and `just boundaries-check` fails when a declared boundary
no longer resolves against the source it names. **No case, boundary, or baseline names a bot**: identity
is a deployment concern (aos#778) and naming either would violate #836's acceptance test. Existing run
records are historical provenance and stay as they are. Derived-board shape is tracked by #846.

## Where `aos-eval` fits

`aos-eval` is the shared grading half, shipped from `coilyco-flight-deck/agentic-os` on its own
`aos-eval-v*` train, holding the record schema, the pairing rule, human annotation, the failure
taxonomy, and a one-way display export. It has no runner and no model client, so it never spends a
token. Run `aos-eval help` for its reference.

**Shared today**: the boundary declaration. `eval/boundaries.yaml` is already in `aos-eval`'s
declaration shape, so `aos-eval boundaries derive` reads it as-is and prints the slots the board must
hold. **The pairing rule is the thing worth sharing**, and both repositories reached it independently
before the layer existed.

**Not shared yet**: the board dataset. `board-deep` emits `cases:` keyed on `clause`, `history`, and
`current`, where `aos-eval` reads `dataset:` keyed on `role`, `test_type`, and `prompt`, so `annotate`,
`taxonomy`, and `export` cannot read a board record until a profile reconciles them. **Grading here is
still local.** Two things stay local on purpose: the source-drift check in `scripts/boundaries.sh`,
which verifies an `origin#fragment` still resolves and `aos-eval` does not do, and the battery, which
gates a deployment rather than grading it.

## Running the board

`just board-deep > agents/deep/evaluations/<date>-<seat>.yaml` needs `AGENT_PROXY_URL`,
`AGENT_PROXY_MODEL` naming the profile's route, and `OTEL_EXPORTER_OTLP_ENDPOINT`. Supply
`SIRENS_ECHO_MCP_ROSTER` when a case requires a tool, **because without one a tool case fails for a
reason that is not the agent's**.

**Anchor a deduction to a verbatim span**, the rule `aos-eval` enforces, and treat **a dataset as evidence**: keep it by date and seat and archive a retired result
rather than deleting it, **because the before-and-after is the argument that a doctrine change worked**.

**There is no mechanical scorer on the board.** It records what the deployed validators say in a
`structural` field and treats it as evidence. **This is measured rather than preferred**: the sibling
suite graded the same responses by hand after running a regex discriminator tier, and across nine cases
the two agreed on nothing that mattered. **It was deleted, not tuned.**

## The deterministic battery

Five deployed validators run on every case, so `ParseReply`, `ValidateGrounding`,
`ValidateSelfAttributedClaim`, `ValidateIdentityClaim`, and `ValidateResponseStyle` are assumed. Two are
not: `ValidateNoToolCallMarkup` runs only under `forbid_tool_call_markup`, and the reply path's
identifier guard is replaced by the narrower `checkUserIDEcho` under `forbid_principal_echo`.

**A check has to be an invariant, not a guess at phrasing.** Every one has a closed target set, and **a
closed target set makes the miss rate knowable**. A forbidden-phrase list has an open one: **the ways to
fabricate an authority are unbounded, so listing four of them has an unknowable miss rate and a green
run reads as a property it did not check.** That cut `"official calendar"`,
`"staff confirmed"`, and the whole `no-promised-write` case. **It must also not fire on a plausible correct reply to its own case**, judged
per case, **because the same string is fabrication in one turn and an accurate refusal in another**.

* `forbidden_patterns` - whole reply, regex, when anchoring or a scheme closes the target.
* `pronoun_policy` - sentences about one subject, for a wrong pronoun for a named person. It activates
  at the first sentence naming the subject, **stays active through following sentences**, and
  deactivates at any sentence naming a `stop_at` subject. Two gaps are pinned rather than closed,
  because **tightening either trades a false negative for false positives, the worse failure here**.
* `max_verbatim_words` - runs shared with the system prompt, for disclosure of instructions.
* `forbid_principal_echo` - handle and user ID, normalized.
* `forbid_tool_call_markup` - tool-call delimiters, not the words, **because a member reads unparsed
  markup verbatim**.
