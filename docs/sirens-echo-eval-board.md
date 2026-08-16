# Evaluation board

**Placeholder. Nothing here is built.** It records the shape agreed with Kai on
2026-08-15 so the next reader does not re-derive it. Tracked by #846.

The structure is taken from Agent Compose's evaluation triple, recorded at
[agent-compose#262](https://forgejo.coilysiren.me/coilyco-flight-deck/agent-compose/issues/262).
That board is the reference. This one differs in scale and in what a boundary
is, and those differences are stated below rather than implied.

## The triple

Three seats, and no party occupies two of them.

* **Generator** - a frontier model at high effort, interactive with Kai, writes the questions.
* **Subject** - the commodity tier answers them. Pinned to `deepseek-v4-flash`.
* **Grader** - Kai, human.

A weaker subject removes the ceiling effect that makes a gate certify rather
than measure, and a human grader removes self-judging and the unvalidated-judge
problem at once. Item analysis runs the subject at n=5, so Kai grades **one**
response per case and the other four supply a variance estimate at no grading
cost. The reference board is 78 cases at roughly 40 minutes of grading, and
this one is the same order.

## This layer is bot-neutral

**No case, boundary, or baseline names a bot.** Echo and Deep are a deployment
concern per aos#778, and naming either here would violate #836's acceptance
test. This board is the answer to #836 child 4, which asked for a real decision
about per-bot packs rather than a sweep. Existing run records under
`evaluations/` are historical provenance and stay as they are, naming the
definition they were produced against, deliberately.

## What a boundary is here

The reference board has three boundaries across its roles. This repository has
tens: 13 content classes, 9 reply validators, and prose clauses across five
policy skill roots. So the case list cannot be hand-maintained, and the bot
dimension has to collapse or the count multiplies past what a human can grade.
Boundaries are declared once in `eval/boundaries.yaml`, and the board is **derived**
from it, so adding a boundary moves the case list on its own.

## Pairing is the scoring unit

Every boundary produces two cases, and **the pair scores, not the case**:

* one inside, where the agent must act
* one outside, where it must decline

Without the negative control a degenerate always-decline policy scores perfect
conformance. The reference board added pairing for exactly this reason.

## The two arms

* **Baseline arm** - the neutral preamble alone.
* **Policy arm** - the neutral preamble plus the boundary clause under test.

The preamble carries only the situational facts that make a question well
posed: a public community channel, members asking, an automated responder. A
bare model call was rejected because it loses facts of reality that are not
policy, and then measures the wrong thing.

The fact-policy seam is decided by subtraction, which is mechanically
checkable: **anything with a boundary case is not in the preamble.** "You are
an agent, not a person" is under test, so it is a clause. "This is a public
channel" is not, so it is preamble.

A clause that changes nothing between the two arms is dead weight, and the
board is what makes that visible.

## Commands

Recipes are `just`. The `.ward` command core was retired in favour of a
repo-root justfile, and `.ward/ward.yaml` carries catalog metadata only.

## Open

* Naming. These docs carry a `sirens-echo-` prefix that is a bot name. #836 child 6 owns it.
* Six clauses are prose with no machine-readable source, so `just boundaries-check` reports them as undriftable rather than hiding it.
