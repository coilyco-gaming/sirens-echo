# The rendered prompt

The model's instruction surface comes from three tracked sources, and `agent/rendered/*.prompt.txt`
holds the assembled result.

`internal/community/prompt.go` supplies the scaffolding: harness identity line, pronoun policy, identity
policy, admission sentence, trust policy, untrusted-input clause, tool-use clause, reply contract,
issue-draft policy, and the neutral style block. Sections join with a blank line and an empty one drops
out, so a social profile renders none. `agent/*.yaml` selects identity, response style, channel label,
issue tracker, and which policy roots load. `.agents/skills/<root>/SKILL.md`, or `COMPOSED.md` for a
composed source, plus one level of `references/*.md`, supplies the rest: `LoadSkillpack` collects across
every configured root, sorts by full path, strips frontmatter, and joins with `## Source: <path>`
headers under a 256 KB cap. **Deployment selects which definition loads and contributes no prose.**

Every profile opens by naming its identity, the sirens-echo harness, and the Coilyco Gaming Robotics
Division, then carries the pronoun policy, the admission sentence, and the trust policy. That policy
**names Kai as the only trusted speaker and treats every other input as a passive threat probe**.
Deployment supplies her Discord handle and user ID through `SIRENS_ECHO_PRINCIPAL_HANDLE` and
`SIRENS_ECHO_PRINCIPAL_USER_ID`, and the same paragraph denies those two signals any grant of their own:
**a blanket grant exists only in a direct message with her**. Set both variables or neither, since
naming no principal renders no identity signals, **which trusts nobody rather than the wrong somebody**,
and the validator rejects a prompt naming a principal deployment did not configure. A profile naming a
channel adds its Discord boundary to the admission sentence, and `ValidateSystemPrompt` fails the build
when any of that goes missing.

## What a reply is answering

A member replying to a message is addressing that message, so the turn names it rather than leaving the
model to infer it from position: `bob is replying to alice: the plank market crashed on tuesday`. **The
recent conversation still renders in full**, because naming the subject supplements recency rather than
replacing it. Discord delivers the addressed message inline for most replies, and otherwise the harness
fetches it under the same budget as the other gate-forced calls. **Only one level renders**: a reply to
a reply does not walk the chain, because the second is a claim about someone else's subject.

**The turn also carries a clock.** Nothing named the time, so `what time is it` had no answer and
`discord-timestamps` had no epoch to render from (#855). One system message states the moment, **read
once per turn**, so two rounds agree about it.

## Snapshots

The rendered prompt is checked in, so a change to what the model is told shows up as a diff rather than
as a behaviour someone notices later. `just prompt-dump` rewrites the snapshots and `just prompt-check`
fails when one is stale, the latter also a pre-commit hook over `agent/`, `.agents/skills/`,
`prompt.go`, `skillpack.go`, and the dumper, **so a prompt change cannot land without its diff**.

Each snapshot carries the definition path, identity, response style, policy roots, and system-prompt
byte count, then the system prompt, turn context, and user message from a fixed sample. **The sample
keeps those sections deterministic, so a diff there means the framing changed.** The turn is three
messages: system prompt, the conversation around the request as its own user turn, then the member's
message alone. **History stays flattened and labelled inside the context message**, because a Discord
channel is multi-party and the assistant and user roles cannot say which human spoke.

The files are byte-exact, so `trailing-whitespace` and `end-of-file-fixer` skip `agent/rendered/`, and
editing one by hand is pointless because the hook regenerates from source. **A diff there is the honest
answer to "what did this change tell the model"**: read it before approving a change to any policy root,
since a one-line `SKILL.md` edit can move hundreds of bytes. The header byte count is **the only place a
skill root's per-turn cost is stated as a number**.

## The system prompt is not a secret

Three checks treated it as confidential. **It is not**, and this records the decision so nobody re-adds
them believing otherwise. `max_verbatim_words` compared a reply against the system prompt and failed on
eight consecutive shared words; it is retired, with the `prompt-leakage` gating case that existed only
to run it.

It was never a real control: the prompt is assembled from policy roots and capability references tracked
in this public repository, so **a check against a public document is theatre, and expensive theatre,
because it gated deployments**. It also failed in the direction that costs most, because **a correct
refusal frequently describes what the service can do in the words the prompt used**, since that is where
the words came from. So it fired on compliance and not on extraction, and **a security row that is red
for correct behaviour teaches readers to skip the row that finally matters**.

Configuration identifiers are still checked: `SIRENS_ECHO_*` names are not secret either, **but reciting
them is a shape no correct reply has**, and the pattern costs nothing. The operator's user ID remains
forbidden, being member data rather than prompt confidentiality. It follows that **a reply that quotes
the prompt is not a defect, and neither is one that lists the tools the service offers** - that question
was open and this answers it. If either is undesirable it is a *composure* concern about a service that
volunteers more than it was asked, which needs to be made on its own terms.

## Prompt budget

`TestRenderedPromptsStayInsideTheirBudget` bounds each tracked snapshot. **The numbers in
`promptBudgets` are a ratchet, not a target.** Every turn ships the whole system prompt, so growth is a
per-turn cost paid for as long as the profile runs, **and it is invisible in a diff that adds ten
reasonable lines to a policy root**. The Echo prompt went from 6918 bytes to 16962 in a single evening
across four changes each defensible on their own. **Nobody chose 16962**, and that is the failure this
prevents: not a large prompt, but one that arrived without a decision.

When a change pushes a snapshot past its budget, the test names the file, the actual size, and the
ceiling. Raise the number and say in the commit message why the bytes are worth it, or trim a policy
root: **silently growing is the only outcome this removes**. The budgets carry headroom on purpose,
because **a test that fails on every ordinary edit trains people to raise the number without reading
it**. **It does not measure cost** - a byte count is a poor proxy for tokens across tokenizers and says
nothing about the cache behavior tracked in the prompt-caching issue - **and it does not judge
content**: a registry of complete URLs is larger than a template the model fills in, and larger on
purpose, because a model with no closed list invents addresses.

Every raise is recorded with its cause, because **a raise is only correct when the growth was
intended**: Echo to 21976 for composing the ops role; Deep to 12260 when it gained
`issue_tracker: forgejo`, the whole 863 bytes being the filing block Echo already carried; both by 315
when Kai widened the filing trigger on #235; Echo to 23826 for object emoji (#203), the whole 1535 bytes
being `references/object-emoji.md`; both by 1951 when the org relationship became a shared knowledge source (#806); both by 5
when giving every agent one folder (#816) lengthened the definition path in each snapshot's header; and
both by 203 for the rule routing arithmetic through the calculate tool (#916), which is what makes
that tool used rather than offered.
**Recording a header change is pedantic, and that is the point.**
