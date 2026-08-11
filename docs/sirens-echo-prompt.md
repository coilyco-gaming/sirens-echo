# Rendered prompt

The model's whole instruction surface is assembled from three tracked sources.
`agent/rendered/*.prompt.txt` holds the assembled result, so a prompt change
shows up as a reviewable diff rather than something a reader has to concatenate
by hand.

## Sources

* `internal/community/prompt.go` - the scaffolding: harness identity line,
  pronoun policy, admission sentence, trust policy, untrusted-input clause,
  tool-use clause, JSON response contract, issue-draft policy, and the neutral
  style block in `responseInstructions`. Sections join with a blank line and an
  empty one drops out, which is how a social profile renders no style block.
* `agent/*.yaml` - selects identity, response style, channel label, issue
  tracker, and which policy roots load.
* `.agents/skills/<root>/SKILL.md`, or `COMPOSED.md` for an agent-compose
  composed source, plus one level of `references/*.md`. `LoadSkillpack` collects
  across every configured root, sorts by full path, strips frontmatter, and
  joins with `## Source: <path>` headers under a 256 KB cap.

Deployment selects which definition loads and contributes no prose.

## Shared framing

Every profile opens by naming its identity, the sirens-echo harness it runs on,
and the Coilyco Gaming Intelligence Team, then carries the pronoun policy, the
admission sentence, and the trust policy.

The trust policy names Kai as the only trusted speaker and treats every other
input as a passive threat probe. It supplies her Discord handle and user ID and,
in the same paragraph, denies those two signals any grant of their own: a
blanket grant exists only in a direct message with her.

A profile naming a channel adds its Discord boundary to the admission sentence.
A channel-less profile asserts no ingress the deployment did not select.
`ValidateSystemPrompt` fails the build when any of that goes missing, so a
rendered prompt cannot lose the trust boundary quietly.

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
