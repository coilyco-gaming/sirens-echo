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
| `substrate` | Host state at run time, free text, set through `SIRENS_ECHO_SUBSTRATE`. |
| `image` | A deployed image, only when one participates. |
| `composed` | Whether the agent-compose bundle was real, `stubbed`, or `absent`. |

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

**`composed: stubbed` bounds every Deep number here.** A composed definition
reads a placeholder of a few hundred bytes where a deployed pod injects the real
agent-compose bundle, so an eval run and production differ in their
instructions and not only in their build. The stub is deliberate and keeps the
tracked snapshot hermetic. The field exists so a reader can tell, since the
cases most likely to move under a larger prompt are the ones these packs
measure. The runner sets it from the prompt it built, so a caller cannot claim
a bundle a run never read.

`runner` is filled automatically by `scripts/ward-command.sh` from the current
checkout, so it is present without anyone remembering it. `SIRENS_ECHO_RUNNER`
overrides it, and a build carrying a `-X` revision stamp takes precedence over
both. Provenance that depends on a human remembering is provenance that goes
missing, which is how the first datasets were filed with the SHA hand-written
into `substrate`.

## See also

- [the rate pack](sirens-echo-rate.md) - how a run is scored and promoted.
