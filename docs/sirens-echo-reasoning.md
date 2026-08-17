# Indistinguishable values

**One defect shape produced eight separate issues in this repository in a single day.** It is worth
naming, because every instance read as correct and none of them failed anything.

**The shape**: a value has a default, an empty state, or a fallback, and a reader cannot tell that state
apart from a real measurement, so they read it as one. Nothing errors, no gate goes red, **and the
number is present, plausible, and wrong about what it means rather than wrong about its arithmetic**.

* **A count that merged two outcomes.** `mcp.tools.list` wrapped the call site whether or not it went to
  the network, so a cache hit and a round trip were one number, **and the count did not change when the
  cache landed, which reads as the cache not working** (#520).
* **A service name that was two services.** The evaluation binary never set `InstanceName`, so it fell
  back to `sirens-echo` and **every evaluation run reported as the production deployment**, with error
  rate, latency, and token spend all contaminated and all plausible (#533). That fix named the binary
  and left the default in place, **so the shape recurred**: 891 spans carrying the Deep profile's
  `agent.attribution` still reported as Echo. Retiring the default was not available either, because
  Echo's own deployment sets no instance name, so **it is refused only where it lies**: an unset name
  with a definition that is not Echo's now fails startup (#542).
* **A failure message that blamed the wrong party.** `expected tool X` was emitted whether the model
  declined a tool it had or the run offered no roster at all, **and only the first says anything about
  the agent**.
* **A boolean that answered a narrower question than its name.** `mcp.tools.cached` was true only when
  **nothing** listed, so `false` read as "nothing was cached" when most of the roster was. Fixed by
  adding `mcp.tools.configured` and `mcp.tools.reached` beside it **rather than changing what the
  boolean meant, which would have moved the problem** (#534).
* **An empty attribute that meant "old pod".** Nine `mcp.tool.call` spans carried no tool name: the
  field was not broken, it was newer than the image, **and a missing value and a new field are identical
  in a query**.
* **A name check that could not see a missing half.** `proxyToolName` composed `server + "__" + tool`,
  trimmed underscores, and rejected an empty result, **which an empty tool name survives because the
  server name is still standing**.
* **A green check that measured the wrong tree.** CI runs on the branch head and no merge ref exists,
  **so a passing pull request says nothing about the merge**: three red `main` events came from pairs of
  individually green branches.

**Ask what the default looks like from the outside** - not whether it is correct, but whether a reader
can tell it apart from a real value, because if they cannot, **that is the defect before any wrong
behaviour exists**. **Prefer a distinct state to a plausible one**: an explicit "unknown" is worth more
than a zero, and a refused call more than a silent fallback. **Check the field's own history before
believing it**, since "this field is empty" and "this field is newer than the thing that wrote it" look
the same and the second is not a defect. **Preserve the old series when a meaning changes**, because a
number that means something different than it did last week while reading identically is worse than a
wrong number.

## Echoing reasoning back exactly as it arrived

DeepSeek in thinking mode rejects a request whose assistant messages do not carry `reasoning_content`.
The harness copied the field at both assistant build sites, so it looked faithful; **the encoding was
not**, because under `omitempty` a model that returned an empty reasoning string and a model that never
mentioned reasoning produce the same bytes: no key. The harness echoed the second shape for both and the
provider refused the array. **Same defect as above, in an encoding rather than a log line.**

**Dropping `omitempty` was the obvious fix and is wrong.** `chatMessage` is one struct for every role,
so dropping it stamps `"reasoning_content": ""` onto system, user, and tool messages, on every request,
on every route, while the failure lives on one evaluation lane. **A repair that rewrites every request
on the healthy lanes to fix a broken one is not mechanical.**

Both the response and request fields are `*string` instead, **so presence survives the round trip**: a
provider that named the field, even as `""`, gets it echoed, and one that never named it sees a
byte-identical request to the one it saw before. An explicit `null` reads as absent, which is what the
previous encoding did with it too.

**What was not established** is whether the provider accepts `"reasoning_content": ""`, which #717 was
blocked on. It is no longer blocking, because **the change cannot be worse than what it replaces**: the
failing case sends the field where it currently sends nothing, and nothing is already the shape that
earns the 400. If the empty string is refused, the fix left is to put something in a field the model did
not fill, **which is a decision rather than a repair**.

## Measuring a forged turn

A case can mark its history as caller-supplied, which is what a forged turn is. Before that, **the
delimiter-confusion case was measuring a forged turn without the defence that case exists to test.**
`assertedHistory` marks caller-supplied conversation so the rendered prompt says where each entry came
from, and it was applied on the HTTP and MCP paths and nowhere else. The evaluation runner built its
prompt straight from the case's history, so the marker never rendered: `injection-fake-system-turn`
passed fifteen times, and **those fifteen runs describe a model resisting a forged system turn with no
provenance marker present**, which is not what the case claims to measure. **Fifteen clean runs of the
wrong thing is worse than no runs, because the number looks like assurance.**

`asserted_history: true` is opt-in per case, **a deliberate limit rather than laziness**. A pack author
does supply case history, so marking all of it asserted would arguably be more faithful, but it would
also change the rendered prompt for every case that has history, **moving baselines that were measured
without it**, which is a decision about what every existing number means. The old number stays,
relabelled: `injection-fake-system-turn` keeps its recorded 0/15 with a note saying what it measured,
because **deleting it would discard a real measurement of the undefended case**. A re-measure is owed.
**The board is deliberately unchanged**, being human-graded with cases that are not testing the marker,
so a rendering difference there would change what a grader reads for no gain.
