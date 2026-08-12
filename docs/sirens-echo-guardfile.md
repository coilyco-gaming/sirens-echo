# Knowing its own authority

Sirens Deep can describe what it is allowed to do, accurately, without being
handed prose that drifts from the deployed boundary.

## Why it is generated

A hand-written skill describing a guardfile is a description of a document, not
a description of live authority. The two drift the moment either changes, and an
agent confidently misdescribing its own boundaries on a permanent public
recording is worse than one that declines to describe them.

So the skill is generated from the KDL. `ward exec guardfile-skill` parses the
deployed guardfile and writes
`.agents/skills/coilyco-general/references/guardfile.md`, which is inside Deep's
`local_skill_roots` and therefore reaches the prompt like any other local policy.

## Deny by absence is the interesting half

The agent already sees its granted tools as schemas. What a tool list cannot
show is the shape of the denial:

* every path is fixed to one repository, with no owner or repository argument to
  redirect, so it cannot be pointed elsewhere by asking
* edits and deletes are denied **by absence**, not by a rule saying no

Absence is the part worth teaching, because there is no exception to find and
nothing to argue with. The generated skill states it as such, and tells the
agent to say "I do not have it" rather than reason about whether something
unlisted would be permitted.

## Staying honest

The rendered skill carries a digest of the guardfile it came from, and
`ward exec guardfile-skill-check` fails when the tracked skill no longer matches
the source. A guardfile edit that does not refresh the skill fails rather than
going quietly stale.

## The cross-repository seam, stated plainly

The guardfile lives in `coilyco-bridge/deploy`, which is private, and this
repository is public. So the check cannot run in this repository's CI: the
source is not reachable from the image build, and vendoring a copy here would
recreate the drift the generator exists to remove.

That means the verification has to run **where the guardfile changes**. The
generator takes a `--guardfile` path so it can run from either side, and the
deploy-side hook is a handoff rather than something this repository can enforce
alone. Until that lands, drift is caught by whoever runs the check, not by a
gate, and saying otherwise would claim a boundary that is not held.

## No new authority

This is knowledge about authority, not authority. No MCP server, no tool grant,
and no write surface is added. See `coilyco-bridge/deploy#365` for the grant
question, which is deliberately separate.

Four layers hold the boundary: network controls, the ward KDL, harness config,
and prose instruction. The KDL is the layer worth teaching, because network
controls cannot be shown, harness config is YAML anyone could have written, and
prose is the mechanism that does not hold under pressure.

See [tools](sirens-echo-tools.md) and [the prompt](sirens-echo-prompt.md).
