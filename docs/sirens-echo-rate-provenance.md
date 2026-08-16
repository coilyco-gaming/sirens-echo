# Rate provenance

What each field in a rate dataset records, and what it can be trusted to say.
See [the rate pack](sirens-echo-rate.md) for how a run is scored.


A rate is only comparable to another rate if the dataset can say what produced
it. The fields that decide that are covered by test rather than by whoever last
edited the struct.

| Field | What it says |
| --- | --- |
| `runner` | The checkout that produced the prompt, definition, skillpack and checks. This is what actually produced the numbers. |
| `roster` | The MCP roster, or `empty`. |
| `fixture` | The tool fixture, or `none`. Exclusive with the roster. |
| `substrate` | Host state at run time, free text, from `SIRENS_ECHO_SUBSTRATE`. |
| `model` | The proxy model **group**. See [model groups](sirens-echo-model-groups.md). |
| `composed` | Whether the agent-compose bundle was real or a stub. |
| `image` | A deployed image, only when one participates. |

**`image` stays `unrecorded` for every run of this instrument**, and that is
correct rather than a gap. `cmd/sirens-echo-eval` assembles the prompt locally
and posts to the proxy, so no pod takes part in a run. Read `runner` instead. A
number attributed to an image that had no part in producing it is worse than a
number with no image at all.

**`roster: empty` on its own does not mean no tools.** A fixture run reports an
empty roster and still serves tools, which is the whole point of the fixture. The
pair to read is `roster` and `fixture` together. Getting this wrong turns a
meaningful result into a vacuous one: the data-borne injection cases only mean
something if the payload actually arrived in a tool result.

`runner` is filled automatically by `scripts/task.sh` from the current
checkout, so it is present without anyone remembering it. `SIRENS_ECHO_RUNNER`
overrides it, and a build carrying a `-X` revision stamp takes precedence over
both. Provenance that depends on a human remembering is provenance that goes
missing, which is how the first datasets were filed with the SHA hand-written
into `substrate`.

## See also

- [the rate pack](sirens-echo-rate.md) - how a run is scored and promoted.

## The composed bundle is stubbed, and that bounds every Deep rate

`agents/deep/definition.yaml` sets `composed: true`. The runner substitutes
`PlaceholderComposed`, which is **249 bytes**, where the deployed pod injects the
role skill, the personality skills and the rest of that context. When
that prompt was last measured in production it was 53,133 bytes against the
11,392 the snapshot renders, so the bundle was most of it.

So a Deep rate describes **this configuration**, not the deployed service, and in
a stronger sense than "a different image": the instructions differ. A model given
11 KB is not obviously the same subject as one given 53 KB.

The stub is the default and stays, because the snapshot and `policy-check` must
stay hermetic. Those are build-time paths and a rate is not one, so
`SIRENS_ECHO_COMPOSED_BUNDLE` points a run at a staged bundle and `composed`
names it. An unreadable bundle fails the run rather than falling back: a dataset
naming a bundle it never read is worse than the gap. This closes the instruction
gap, not the build gap, since no pod participates either way. **What was missing was any statement in
the dataset that the stub was used**, which is the difference between a bound a
reader can see and one they cannot.

`composed` is derived where the substitution happens rather than passed in by the
caller, so a dataset cannot claim a real bundle when the run used the stub. A
caller-supplied value is overwritten, and a test pins that.

Echo is unaffected. `sirens-echo.yaml` is not composed, so its 20,397 byte
snapshot is what a turn actually ships.

Whether an eval run should be able to load a real bundle is
[sirens-echo#316](https://forgejo.coilysiren.me/coilyco-gaming/sirens-echo/issues/316)
part 2, and it is a runner decision with a hermeticity cost rather than a
provenance one.
