# Testing and the lanes

The pre-push gate, what a skip costs, how a sweep stays comparable, and how a merge names its work.

## The pre-push gate

`just gate` runs everything CI runs, in CI's order: `build`, `policy-check`, `vet`, `test`,
`test-skips`, `pre-commit`. **It exists because five separate habits is four too many**: `vet`, `test`,
and `build` all pass on a tree that CI rejects, **so an engineer who runs the obvious verbs still learns
about a violation from a red `main`**. **Pre-commit runs last because its hooks are fixers** that
rewrite files, **so running it first would check a tree that no longer exists by the time the others
finish**. **It does not stand in for the commit hook**: `git commit` does not invoke `pre-commit` unless
someone has run `pre-commit install`, and this repository is worked from task-scoped temporary clones
that start without hooks. `just setup` installs them and `scripts/task.sh` installs a missing hook
before running any verb it carries, though recipes naming their tool directly in the justfile never
reach that code (issue 305).

**A file git has never seen.** Several hooks enumerate git's own file list rather than the paths
pre-commit hands them, **so `--all-files` means every *tracked* file**, and a file that has never been
added is invisible to them, **which made the gate green on a tree the commit then rejected**. The new
file is the most likely thing in a change to carry a fresh violation, **so this was the case the gate
most needed to cover and the one it silently did not**. The gate therefore marks every untracked,
non-ignored file with `git add --intent-to-add` before the hooks run, **intent only, no content
staged**, and the marks are removed however the run exits **so a red gate leaves the index exactly as it
was found**.

## Every skip is a reviewed line

**A skipped test and a passing test print the same word and exit the same way**, and nothing counts how
many tests ran, **so a guard can stop running for months without anyone learning**. Two guards were
found unguarded at once, and deleting the thing each protected left the suite green. **The sharper case
was disabled by a correct, unrelated fix**: every other reference to a renamed path was updated, **and
only the one wrapped in a skip swallowed its error rather than failing**.

**The check runs on what fires, not on what is written.** A source scan for `t.Skip` would not have
caught either, because both carried their skip from the day they were written: **what changed was the
skip starting to fire**. So `just test-skips` runs the suite verbose, collects the tests that actually
emitted a skip, and compares that set to `scripts/test-skips.allow`. **It fails in both directions**: an
unreviewed skip fails, and **a stale allowlist entry fails too, which is not optional**, since an entry
that no longer fires reads as a known exception forever **and nobody deletes a line that is not
hurting**. **An allowlist rather than a count**, because a count is a number someone bumps **and a name
is a line someone has to write a reason next to**.

## Tool fixtures

An evaluation case can set what the caller says. **It cannot set what a tool returns**, so no case could
reach a payload carried inside tool output. Player-authored strings already arrive as tool results, in
store, currency, and settlement names and handles, so **writing only chat-box cases and then landing a
wider ingest on the strength of them is assurance from tests that do not test the thing**.

`FixtureProvider` implements `ToolProvider` in process, opening no socket and starting no process. A
declared result is returned from `Call` and flows into the message list **through the same path a real
MCP result takes**, so there is no second injection point and the context assembly under test is the
production one. `SIRENS_ECHO_TOOL_FIXTURE` selects it and is exclusive with `SIRENS_ECHO_MCP_ROSTER`,
**because a run reaching both could not say which surface answered**. **Arguments are ignored**, because
varying a result by argument would make the payload conditional and the case fragile. **An undeclared
tool returns an error rather than an empty result**, since a silent empty string would let a case pass
while measuring nothing, and `Grounding` and `Unavailable` are empty rather than synthesised **so a case
cannot mistake fixture state for roster state**. There is deliberately no transport realism.

**The rule that has no code: never create an Eco store, currency, or settlement named with a payload to
make a live case work.** That mutates a world a community plays in, to run a test.

## Sweep run protocol

A multi-model sweep is **only a comparison if the substrate was equal**. The local GPU tier becomes
unusable when the host is doing anything else, **and nothing in the path detects it, routes around it,
or reports it**. **End-state scoring cannot tell a starved backend from a model that genuinely cannot do
the task**, so a self-hosted cell run on a contended host concludes that the open-source tier cannot do
the behavior - **the finding the sweep was looking for, arrived at for the wrong reason, and believable
because it matches the prior**.

Run the self-hosted cell only against a host known to be idle, and **verify before the cell rather than
after, because after is an alibi rather than a control**. Record host state for every cell in the run's
own provenance. **A cell that hits a saturated backend or a deadline is void**: re-run it. **Never
compare a cell to another taken under different host conditions.**

The rate runner separates an `error` outcome from a `fail` and reports a case whose attempts all errored
as `measured: false`. **That covers the substrate failing loudly and cannot cover the case that matters
most**: a contended GPU returns a slow but complete reply, raising no error, **which is why the
host-state record is load-bearing rather than the void rule**. `SIRENS_ECHO_SUBSTRATE` and
`SIRENS_ECHO_IMAGE` are copied verbatim into the provenance, and unset, either records `unrecorded`,
**deliberately rather than as a neutral default**. **The image matters more than it looks**: roughly
half of main's pushes publish none, **so the deployed build is often older than the change under
measurement, and a dataset that does not name it still looks entirely legitimate**.

## The merge lane

`ward agent director merge` refuses a pull request whose body carries no same-repo closing reference,
**because the reference is what makes a merge name the work it finished**. There is no Ward verb for
opening one and the Forgejo MCP exposes reads only, so a seat pushes to the AGit magic ref:

```
git push origin HEAD:refs/for/main -o topic=<short-topic> -o title="<pr title>" \
  -o description="closes #N - <one line>"
```

The named branch push is then unnecessary, **because the pull request head is `refs/pull/<N>/head`
rather than a branch**, and pushing both leaves a branch beside the pull request carrying the same
commit. **A push option cannot contain a newline**, so the description is one line and the detail
belongs on the issue - **and that one line is the pull request body, so it is where the closing
reference has to appear**. Accepted: `closes #324` or `closes owner/repo#324`, with `fixes` and
`resolves` equivalent, the keyword followed immediately by the reference.
