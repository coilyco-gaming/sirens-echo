# Rendered prompt

The model's instruction surface comes from three tracked sources.
`agent/rendered/*.prompt.txt` holds the assembled result, so a prompt change is
a reviewable diff rather than something a reader concatenates by hand.

## Sources

* `internal/community/prompt.go` - the scaffolding: harness identity line,
  pronoun policy, [identity policy](sirens-echo-identity.md), admission
  sentence, trust policy, untrusted-input clause, tool-use clause, reply
  contract, issue-draft policy, and the neutral style block. Sections join with
  a blank line and an empty one drops out, so a social profile renders none.
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
admission sentence, and the trust policy. That policy names Kai as the only
trusted speaker and treats every other input as a passive threat probe.
Deployment supplies her Discord handle and user ID through
`SIRENS_ECHO_PRINCIPAL_HANDLE` and `SIRENS_ECHO_PRINCIPAL_USER_ID`, and the same
paragraph denies those two signals any grant of their own: a blanket grant
exists only in a direct message with her.

Set both variables or neither. Naming no principal renders no identity signals,
which trusts nobody rather than the wrong somebody, and the validator rejects a
prompt naming a principal deployment did not configure. The snapshot and `ward
exec policy-check` render a placeholder, as
`docs/access-policy.reference.yaml` does for deployment-owned snowflakes.

A profile naming a channel adds its Discord boundary to the admission sentence,
and a channel-less profile asserts no ingress the deployment did not select.
`ValidateSystemPrompt` fails the build when any of that goes missing.

## Snapshots

```sh
ward exec prompt-dump    # rewrite the snapshots
ward exec prompt-check   # fail when a snapshot is stale
```

`prompt-check` also runs as a pre-commit hook over `agent/`, `.agents/skills/`,
`prompt.go`, `skillpack.go`, and the dumper, so a prompt change cannot land
without its rendered diff.

Each snapshot carries the definition path, identity, response style, policy
roots, and system-prompt byte count, then the system prompt, the turn context,
and the user message from a fixed sample. The sample keeps those sections
deterministic, so a diff there means the framing changed.

The turn is three messages. System prompt, then the conversation around the
request as its own user turn, then the member's message alone, so a downstream
reader of "what did the user ask" gets the question. History stays flattened
and labelled inside the context message, because a Discord channel is
multi-party and the assistant and user roles cannot say which human spoke.

The files are byte-exact, so `trailing-whitespace` and `end-of-file-fixer` skip
`agent/rendered/`. Editing one by hand is pointless: the hook regenerates from
source and fails on the difference.

## Reviewing a change

A diff under `agent/rendered/` is the honest answer to "what did this change
tell the model". Read it before approving a change to any policy root, since a
one-line `SKILL.md` edit can move hundreds of bytes of context. The header byte
count is the cheapest signal of an accidental blow-up.

## See also

See [response profiles](response-profiles.md),
[configuration](sirens-echo-config.md), and [the service](sirens-echo.md).
