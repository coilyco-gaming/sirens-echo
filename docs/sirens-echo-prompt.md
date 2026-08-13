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

## What a reply is answering

A member replying to a message is addressing that message, so the turn names it
rather than leaving the model to infer it from position:

```
bob is replying to alice: the plank market crashed on tuesday
```

The recent conversation still renders in full. Naming the subject supplements
recency rather than replacing it, because a member who replies to something old
and then asks an unrelated question is a real member.

Discord delivers the addressed message inline for most replies, which costs no
lookup. When it does not, and that is likeliest for the old messages this is
worth most for, the harness fetches it under the same budget as the other
gate-forced calls. A reference that cannot be read is an ordinary message.

Only one level renders. A reply to a reply does not walk the chain, because the
second level is a claim about what someone else was addressing.

## Snapshots

The rendered prompt is checked in and a change to it cannot land without its
diff. See [prompt snapshots](sirens-echo-prompt-snapshots.md).

## See also

See [response profiles](response-profiles.md),
[configuration](sirens-echo-config.md), and [the service](sirens-echo.md).
