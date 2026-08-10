# Rendered prompt

The model's whole instruction surface is assembled from three tracked sources.
`agent/rendered/*.prompt.txt` holds the assembled result, so a prompt change
shows up as a reviewable diff rather than something a reader has to concatenate
by hand.

## Sources

* `internal/community/prompt.go` - the scaffolding: untrusted-input clause,
  tool-use clause, JSON response contract, issue-draft policy, and the two
  style blocks in `responseInstructions`.
* `agent/*.yaml` - selects identity, response style, channel label, issue
  tracker, and which policy roots load.
* `.agents/skills/<root>/SKILL.md` plus one level of `references/*.md` - the
  domain prose. `LoadSkillpack` collects across every configured root, sorts by
  full path, strips frontmatter, and joins with `## Source: <path>` headers
  under a 256 KB cap.

Deployment selects which definition loads and contributes no prose.

## Snapshots

```sh
ward exec prompt-dump    # rewrite the snapshots
ward exec prompt-check   # fail when a snapshot is stale
```

`prompt-check` also runs as a pre-commit hook over `agent/`,
`.agents/skills/`, `prompt.go`, `skillpack.go`, and the dumper itself, so a
prompt change cannot land without its rendered diff.

Each snapshot carries the definition path, identity, response style, policy
roots, and system-prompt byte count, then the system prompt, then a user prompt
rendered from a fixed sample. The sample keeps the second section deterministic,
so a diff there only ever means the transcript framing changed.

The files are byte-exact, so `trailing-whitespace` and `end-of-file-fixer` skip
`agent/rendered/`. Editing one by hand is pointless: the hook regenerates from
source and fails on the difference.

## Reviewing a change

A diff under `agent/rendered/` is the honest answer to "what did this change
tell the model". Read it before approving a change to any policy root, because
a one-line edit to a `SKILL.md` can move several hundred bytes of model
context.

The byte count in the header is the cheapest signal of an accidental blow-up,
such as a reference file that pulled in far more than intended.

## See also

See [response profiles](response-profiles.md),
[configuration](sirens-echo-config.md), and [the service](sirens-echo.md).
