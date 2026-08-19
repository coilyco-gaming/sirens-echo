# The content gate

`SIRENS_ECHO_CONTENT_CLASSES` names the taxonomy in `agent/content-classes.yaml`, and it is what makes
that file's `deny: true` mean anything at runtime.

**Unset loads none, runs no classifier, and costs nothing**: not a model call, not a decision, not a tag
beyond `content.classified: false`. **A gate that refuses member requests should arrive by a decision
someone made rather than by a binary being rebuilt.** When it is on it costs one extra model call per
turn, **which lands hardest on the cheapest turns**: an ordinary one-call reply becomes two while a
six-round tool turn barely notices.

**A broken gate is not a denial.** A classifier that errors, times out, or answers with a class outside
the closed set leaves the turn unclassified and the turn proceeds, with the failure logged and the span
saying so. **This is the most important property here**: treating a classifier failure as a refusal
turns one broken dependency into a service that refuses everything, **and it would do so while looking
like policy working**.

Four tags: `content.classified` (whether the gate ran at all), `content.class`, `content.approved`, and
`content.sensitive`. **The first earns its place by being false much of the time**, and a turn never
classified carries only that tag, because reporting a class for a decision that never happened would
make the other three lies. **Sensitive is separate from approved on purpose**: it changes the refusal's
shape, not its verdict.

The gate runs after the prompt is assembled and before the answering call, **so a blocked turn spends no
completion budget on an answer nobody will read**, and it runs on the member's own message rather than
the assembled prompt, because what was asked for is the thing being classified.

## Gate failures

A failed gate emits its own span under the turn, named for how it broke: `content.gate.failed.model`
when the classifier call itself failed, and `content.gate.failed.unknown_class` when the call returned
and named a class outside the closed list.

**The suffix is a fixed vocabulary and never model output.** The class the classifier invented is the
tempting thing to put in the name, and it would make the name unbounded: one span name per invented
string, **a grouping key that never groups**. It goes to the log line beside the span, so it stays
recoverable. `content.classified` is false for a failed gate exactly as for one that never ran, because
from the turn's side those are the same fact and the failure span is what tells them apart. Making that
tag three-valued would push the distinction into every consumer of a boolean that reads correctly today.

## Content classes

`agent/content-classes.yaml` is the closed taxonomy a classifier chooses from. **The requirement is a
list covering every content type theoretically possible to communicate, and a denylist alone cannot
satisfy that**: given only the categories to refuse, an ordinary request has nowhere to land, so the
model either forces it into a deny bucket or answers outside the list, **and both look like the
classifier working**. So the allowed classes are enumerated too and `other` is an explicit catch-all.
The loader refuses a taxonomy without one, and refuses one that enumerates no allowed class.

**Sensitivity is a refusal shape, not a verdict.** `deny` decides whether the request is refused,
`sensitive` decides how: an ordinary block names its reason, a sensitive block emits a generic redirect
naming no category, **because saying which rule fired tells the member exactly what to avoid saying next
time**. **Sensitive wins ties**, so a request matching both resolves to the sensitive class: a bedtime
story trips creative long-form and minor suspicion together, and naming the ordinary category out loud
would leak the signal the sensitive branch exists to hide. `Verdict` enforces this regardless of the
order the classifier returns.

Two rules the file carries from elsewhere. **A block is always visible**, so a refusal is never
mistakable for an outage. **A block is one sentence**, because every volunteered justification is a
handle the next message can pull.

**The emotional-support class covers what a member asks for**: comfort, reassurance, validation. It does
**not** cover the separate defect of the service asserting a member's inner state, which is a grounding
failure no topic filter reaches. A member asking whether their message read as hostile is asking a
question about text, ordinary community logistics that must not be blocked, **and a filter that catches
both is worse than no filter**.

**A classifier that is itself a model call inherits the timeout and backend-failure modes of the turn it
guards**, so a cheap deterministic prefilter is worth considering before the inference.

## What a content block says

A block is the reply a member sees while probing a boundary. **That makes it the worst possible place
for unguarded output, and the place where saying less is worth more than saying it well.**

Every reply guard runs on the reply produced by the reply turn: the identifier guard, the identity
check, the response style, and grounding. **A content verdict comes from a different turn and
short-circuits that path**, so its reason would reach a member having passed none of them. That reason
is model text, and left unbounded it could claim to be a person, echo the operator's handle or user ID,
or run to three sentences, **and it appears exactly when someone is testing what the service will do**.

**A sensitive class carries no model text at all.** The whole design is naming no category, so a
model-written reason there is redundant at best and leaks the hidden signal at worst: a sensitive block
is a fixed redirect and nothing else. **An ordinary block may explain itself, briefly**, keeping its
reason when it survives every check the reply path applies plus two of its own: a word cap, and one
line, **because a multi-line refusal reads as an argument and an argument invites the next message**.

**Failing safe means still refusing.** Any reason that is empty, too long, first person, claiming to be
human, or carrying an identifier falls back to the fixed redirect. **It does not fall back to
answering.** It is **not the notice constructor**: `harnessNotice` renders one short technical phrase and
strips everything outside a narrow alphabet silently, so it would remove a mark, flatten a structured
block to a single line, and **still match the notice shape, so tests would pass on a mangled result**.
