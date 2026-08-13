# When a default is indistinguishable from an answer

One defect shape produced eight separate issues in this repository in a single
day. It is worth naming, because every instance read as correct and none of
them failed anything.

## The shape

A value has a default, an empty state, or a fallback. A reader cannot tell that
state apart from a real measurement, so they read it as one.

Nothing errors. No gate goes red. The number is present, plausible, and wrong
about what it means rather than wrong about its arithmetic.

## The instances

**A count that merged two outcomes.** `mcp.tools.list` wrapped the call site
whether or not it went to the network, so a cache hit and a round trip were one
number. The count did not change when the cache landed, which reads as the
cache not working. See [issue 520](https://forgejo.coilysiren.me/coilyco-gaming/sirens-echo/issues/520).

**A service name that was two services.** The evaluation binary never set
`InstanceName`, so it fell back to `sirens-echo` and every evaluation run
reported as the production deployment. Error rate, latency, and token spend
were all contaminated and all plausible. See [issue 533](https://forgejo.coilysiren.me/coilyco-gaming/sirens-echo/issues/533).

That fix named the binary and left `defaultInstanceName = "sirens-echo"` in
place, so the shape recurred: 891 spans carrying `agent.attribution` of the
Deep profile still report as Echo. Fixing one caller does not retire a default
that is also a live service. See [issue 542](https://forgejo.coilysiren.me/coilyco-gaming/sirens-echo/issues/542).

**A failure message that blamed the wrong party.** `expected tool X` was
emitted whether the model declined a tool it had or the run offered no roster
at all. Only the first sentence says anything about the agent.

**A boolean that answered a narrower question than its name.**
`mcp.tools.cached` was true only when *nothing* listed, so `false` read as
"nothing was cached" when most of the roster was. Fixed by adding
`mcp.tools.configured` and `mcp.tools.reached` beside it rather than by
changing what the boolean meant, which would have moved the problem. See
[issue 534](https://forgejo.coilysiren.me/coilyco-gaming/sirens-echo/issues/534).

**An empty attribute that meant "old pod".** Nine `mcp.tool.call` spans carried
no tool name. The field was not broken; it was newer than the image. A missing
value and a new field are identical in a query.

**A name check that could not see a missing half.** `proxyToolName` composed
`server + "__" + tool`, trimmed underscores, and rejected an empty result. An
empty tool name survives that, because the server name is still standing.

**A green check that measured the wrong tree.** CI runs on the branch head, and
no merge ref exists, so a passing pull request says nothing about the merge.
Three red `main` events came from pairs of individually green branches.

## What to do about it

**Ask what the default looks like from the outside.** Not whether it is correct
— whether a reader can tell it apart from a real value. If they cannot, that is
the defect, before any wrong behaviour exists.

**Prefer a distinct state to a plausible one.** An explicit "unknown" is worth
more than a zero, and a refused call is worth more than a silent fallback.

**Check the field's own history before believing it.** "This field is empty"
and "this field is newer than the thing that wrote it" look the same, and the
second is not a defect.

**Preserve the old series when a meaning changes.** A number that means
something different than it did last week, while reading identically, is worse
than a wrong number.

## See also

- [the gate](sirens-echo-gate.md) - what green actually covers.
- [grounding](sirens-echo-grounding.md) - the same shape in replies.
