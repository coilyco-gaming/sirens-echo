# Knowledge gaps and corrections

How Sirens Echo turns an unanswered question or a correction into a tracked Forgejo issue. **A
definition without `issue_tracker` does none of this** and must return `issue: null`, stating
uncertainty in the reply.

For a definition with `issue_tracker`, an unanswered question produces a `knowledge-gap` draft and an
explicit correction a `correction` draft, **values affecting only the title prefix**. The runtime
removes Discord links, mention syntax, and long identifiers from drafts, and requires a summary without
member identity, handles, raw quotes, secrets, or personal details. The reporter calls the guarded
Forgejo MCP to reuse an exact-title open issue or create an ordinary one.

## Linking what a turn observed or filed

**A short reference such as `#233` resolves against no repository once it leaves the channel, and an
issue filed without being mentioned leaves no trace at all.** Prose prompting has not stopped either
habit, so the harness appends the links instead of asking the model to write them: after the response
checks pass, a `Referenced issues:` block carries the canonical URL for a short-form reference whose
number a tool result **in this same turn** returned, and for any issue this turn filed, whether or not
the reply named it. **Every appended URL came back from a tool call, so the block can state no reference
the runtime did not observe**, and a number the turn never observed stays unlinked rather than guessed
at.

The block is service-authored and added after validation, so it carries no first person, no exclamation,
and no emoji beyond an object's own. **A long answer is shortened to make room for it rather than the
block being dropped**, it resolves against the answer that will actually be sent, and a block that still
cannot fit whole is dropped rather than truncated into a broken URL.

**One number can name an issue in two repositories**, and a tool result quotes a sibling repository's
issue often enough that this is reachable rather than theoretical, so **a number observed with two
different URLs is suppressed**: the block promises every URL came back from a tool call, and on a
collision it is the number-to-URL association that would be the guess. The API URL rides along in the
same payload and is skipped, **not being a link a member can follow**, and the created issue is read
from the response's `html_url` rather than matched anywhere in the payload, **so an issue quoted inside
a new issue's body is not mistaken for the issue just filed**.

**A link is not prose**, so the grounding and neutral-style checks mask links before reading the reply:
without that mask the host `coilysiren.me` reads as the pronoun "me" and a fragment such as
`#issue-8117` reads as an invented channel, **which rejected every reply that carried a link**. When a
write fails, `forgejo.issue.failed` carries the failing MCP tool, the HTTP status, and whether the tool
reported the error rather than the transport, **with the remote response body discarded before the error
is built so it never reaches a log**.

## The labels a filed issue carries

Every issue this service files carries a label marking its contents as unverified, and a
`move-to-repo/*` label saying where it belongs. **The harness applies both. The model never supplies
them and cannot omit them.**

**This service files in response to member input**, so the body of an issue it files contains text a
member influenced, and that issue lands in a tracker four agents read and act on: **without a marker,
attacker-influenceable content is indistinguishable from work an agent authored.** The label already
existed and says exactly the right thing, that the issue came in from the live MCP and its inputs are
not safe or verified until the label is removed.

**It is atomic**: the label goes into the create-issue call rather than a second call afterwards, so
there is no window where the issue exists unlabelled. That is why the deployment must also grant the
`labels` field on create-issue, **the two halves being one change**, since without the grant the call is
rejected for carrying a field the guard does not list.

A filed issue also carries one `move-to-repo/*` label. The deployment names the id and sets
`move-to-repo/unknown` unless it knows the home, **so the default state is the honest one: nothing has
determined where this belongs.** Those labels are exclusive in Forgejo, so the tracker keeps one
destination per issue and a later triage label displaces `unknown` without this service doing anything.

**Safe by default**: no configured label id applies nothing, each is independently optional, and an
unparsable or non-positive id applies nothing, **so a typo disables that label rather than attaching a
wrong one**. The labels are **set rather than merged**, because the model does not supply this field and
a value it invented is not a reason to keep one: **a model that could name its own destination could
route its own issue away from the people who read this tracker.** Only the tracker named by the
definition and only the filing verb are covered; comments carry no label.

## The issue tracker surface

Deploy fixes the Sirens Echo Forgejo MCP guardfile to `coilyco-gaming/sirens-echo`. The published
surface includes issue and label reads, issue creation and closing, comment creation, and label add,
replace, and remove. **There is no owner or repository argument to redirect**, and no issue-body edit,
comment edit, delete, reopen, pin, release, pull-request, repository, organization, or account tool.
**The bound is the guardfile rather than the prompt**: a model that decided to write somewhere else has
no verb that accepts a destination.

Naming that server as `issue_tracker` selects the prompt's issue-filing policy, which tells the model to
search by title and then file a sanitized unlabeled issue through the MCP tool it already holds. **There
is no worker-side path and no Forgejo credential in Echo.** The CoilyCo definition names no tracker and
gets the plain uncertainty instruction, so **the two differ in what the prompt asks for and not in what
the model can reach**. **The roster answers what is reachable, and the tracker name answers what is
encouraged.**

## Reviewing a claim

**Review the evidence, not who produced it.** A model label predicts a distribution and the thing under
review is one artifact, so judging by label substitutes a prior for an observation that is already
available and usually cheaper to check than the prior is to justify. **Distrust by label costs extra
scrutiny and is recoverable. Trust by label costs scrutiny that never happens**, and nobody discovers
what they did not check.

**Two agents agreeing is not corroboration when both are language models.** Overlapping training
distributions make a shared error exactly as likely to produce agreement as a shared correct memory, so
concurrence between them carries much less than concurrence between independent observers. **This only
works if a claim carries something re-runnable**: a number, a line, a query, a byte count. A finding
with no reproduction can only be assessed by who said it, **which is how label-based trust becomes the
default even where nobody wants it**. Both this and the anomaly order reduce to one habit: **prefer the
observation you can re-run over the account you were given**, including when the account is convincing
and especially when it agrees with you.
