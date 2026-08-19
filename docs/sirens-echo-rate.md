# The rate pack

`agents/*/packs/rate.yaml`, run with `just rate-echo` and `just rate-deep`. They measure how often an
intermittent behavior happens, **gate no deployment, and are never wired into CI**.

**Neither existing instrument can hold an intermittent behavior.** The battery hard-fails a deployment,
so a case that fails 13 percent of the time makes the gate flaky, and the board is human-graded, **so it
cannot economically run one case fifteen times**. Before this existed, an observed rate lived only as
prose in an issue body, **with nothing regenerating it or noticing a fix taking 13 percent to 4 rather
than 0**.

A case declares its own `runs` and `max_failure_rate`, and each attempt is scored by
`community.ScoreEvaluationCase`, **the same function the gate uses, so the two instruments cannot
drift**: a rate for a check the gate does not apply would measure something nobody enforces. An attempt
passes, fails, or errors, **an error being a failure of the substrate rather than of the agent** and
excluded from the denominator. **The consequence is that a clean verdict can rest on far fewer runs than the case declared**, so the breach line names how many declared runs errored and were excluded, and a `rate.sample.decimated` warning is logged on stderr for **any** case with errors, a case that passed included. Read `errors` beside `attempts` and `runs`. **No error ceiling gates anything**: failing a verdict on an error rate needs somebody to decide what rate is acceptable, which is a live-operations judgement rather than a measurement one. The run exits non-zero when a case beats its ceiling, when every attempt
of a case errored, or when the boundary median is not below the conversational one: **an unmeasured case
is not a passing one**.

**The dataset is the evidence.** Every reply is persisted verbatim, because three first-pass findings in
the QA that motivated this pack were **defects in the check rather than the agent, and only reading the
text separated them**. Provenance travels with it, since a rate without its definition, pack, model, and
roster is not comparable to the next run. **Every failing check is recorded rather than the first.**

A case starts here to establish its rate, and when a fix drives that rate to zero and holds at high N it
may move into the evaluation pack as a deterministic regression. **Do not promote on a small clean
run**: five runs put a weak upper bound on the true rate, since **a behavior at 13 percent passes 5 of 5
about half the time**. Promotion is also where the battery's rules reattach - a promoted case must not
be able to fire on a correct reply, and its target set must be closed. One attempt is one completion
plus up to six tool rounds, **affordable on demand, which is why this is an invoked verb**.

**What it cannot measure.** *Whether a tool told the truth*: checks score the reply and
`ValidateGrounding` scores it against the tools that ran, **but nothing scores a tool result against the
world**, so a tool returning zero for a valid question yields a faithful, grounded, confidently false
answer that every instrument here passes. A fixture cannot close it, **because a fixture declares its
own result**, testing how the model handles a payload rather than whether the payload was right.
*Whether a number describes deployed Deep*: a composed definition reads a placeholder where the pod
injects the real bundle. *A behavior with no deterministic check*: paraphrase disclosure discloses while
quoting nothing, **so no expression separates it from a correct answer**, and that belongs on the board.

## Provenance

A rate is only comparable to another rate if the dataset can say what produced it, **and the fields that
decide that are covered by test rather than by whoever last edited the struct**. `runner` is the
checkout that produced the prompt, definition, skillpack, and checks, **which is what actually produced
the numbers**. `roster` is the MCP roster or `empty`; `fixture` is the tool fixture or `none`, exclusive
with the roster. `substrate` is free-text host state, `model` is the proxy model **group**, `composed`
records whether the agent-compose bundle was real or a stub, and `image` names a deployed image only
when one participates.

**The composed bundle is stubbed, and that bounds every Deep rate.** `agents/deep/definition.yaml` sets
`composed: true`, and the runner substitutes `PlaceholderComposed`, **249 bytes**, where the deployed
pod injects the role skill, the personality skills, and the rest of that context. When that prompt was
last measured in production it was **53,133 bytes against the 11,392 the snapshot renders**, so the
bundle was most of it. **So a Deep rate describes this configuration, not the deployed service**, and in
a stronger sense than "a different image": **a model given 11 KB is not obviously the same subject as
one given 53 KB**.

The stub is the default and stays, **because the snapshot and `policy-check` must stay hermetic**. Those
are build-time paths and a rate is not one, so `SIRENS_ECHO_COMPOSED_BUNDLE` points a run at a staged
bundle and `composed` names it. **An unreadable bundle fails the run rather than falling back**: a
dataset naming a bundle it never read is worse than the gap. `composed` is derived where the
substitution happens rather than passed in by the caller, **so a dataset cannot claim a real bundle when
the run used the stub**, and a caller-supplied value is overwritten with a test pinning that. Echo is
unaffected, not being composed.

## Trace lookup

**A turn is a trace lookup when both halves are present**: the word `trace` on its own word boundary,
and a trace id of 32 hex characters, also on word boundaries. The id can be typed or carried by the
message the member replied to, **the replied-to case being the common one**, and **a typed id wins over
a quoted one**, since a member who types an id has named a different turn. **Both halves are required,
in both directions**: the word alone cannot name a turn and guessing which one would be worse than not
answering, while an id alone is someone quoting a hash, **and reading that as a telemetry request is the
false positive the keyword exists to prevent**.

**Retrieval does not exist**: this service has no tracing backend in its roster, the grant being
deploy-owned (#278). Until then a recognised lookup is recorded as `turn.trace.requested` with
`served: false` and the turn proceeds normally, **so the demand is measured before the fetch is built
rather than assumed**. **A trace id is not a secret, but it is also not scoped to whoever pastes it**: a
turn span carries the author's account id, the channel, and the message id, **so a member pasting
another member's id would be asking this service to read that member's identifiers out in a public
channel**. Three rules are plausible - own turns only, anyone summarised (stage, outcome, timing, never
identifiers), or anyone everything - and **the seam is built toward the first, because a narrow rule can
be widened by a decision and a wide one can only be narrowed by an incident**. The detector reads the
notice this harness emits, and a test asserts the notice is still parseable by it.

## Where a log line goes

Every line goes to two places: stdout, and the SigNoz collector over OTLP/HTTP, **the exported copy
carrying the same resource the traces do, so `service.name` is the same value on both signals**. The
harness previously exported traces and metrics and no logs; lines still reached SigNoz because the
cluster agent scrapes pod stdout, **but a scraped row cannot carry `service.name`**, so those rows were
keyed by `k8s.deployment.name` - and **`service.name` is the key every SigNoz doc and the MCP's own
shortcut reaches for**, matching nothing and returning an empty result rather than an error.
