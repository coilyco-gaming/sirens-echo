# Knowledge gaps and corrections

How Sirens Echo turns an unanswered question or a correction into a tracked
Forgejo issue. A definition without `issue_tracker` does none of this.

For a definition with `issue_tracker`, an unanswered question produces a
`knowledge-gap` draft and an explicit correction produces a `correction`
draft. These values affect only the issue title prefix. The runtime applies no
labels. A definition without `issue_tracker` must return `issue: null` and
state uncertainty in the reply.

The runtime removes Discord links, mention syntax, and long identifiers from
drafts. It requires a summary without member identity, handles, raw quotes,
secrets, or personal details. The automatic reporter calls the guarded
Forgejo MCP's HTTP tool projection to reuse an exact-title open issue or create
an ordinary issue. A reviewed change updates the local skill and regression
case.

## Linking what a turn observed or filed

A short reference such as `#233` resolves against no repository once it leaves
the channel, and an issue filed without being mentioned leaves no trace at all.
Prose prompting has not stopped either habit, so the harness appends the links
instead of asking the model to write them.

After the response checks pass, the runtime appends a `Referenced issues:` block
carrying the canonical URL for a short-form reference whose number a tool result
in this same turn returned, and for any issue this turn filed, whether or not
the reply named it. Every appended URL came back from a tool call, so the block
can state no reference the runtime did not observe. A number the turn never
observed stays unlinked rather than guessed at.

The block is service-authored text and is added after validation, so it carries
no first person, no exclamation, and no emoji of its own. A block that cannot
fit the send budget is dropped rather than truncated into a broken URL by the
transport.

The API URL for an issue rides along in the same tool payload and is skipped,
because it is not a link a member can follow. The created issue is read from the
response's `html_url` rather than matched anywhere in the payload, so an issue
quoted inside a new issue's body is not mistaken for the issue just filed.

A link is not prose, so the grounding and neutral-style checks mask links before
reading the reply. Without that mask the host `coilysiren.me` reads as the
pronoun "me" and a fragment such as `#issue-8117` reads as an invented channel,
which rejected every reply that carried a link.

## Claiming a filing that did not happen

The grounding check rejects an action claim the runtime did not perform, but its
first-person matcher reads only `I filed` and its siblings. The neutral profile
forbids all first-person voice, so for a neutral definition the two contracts
never overlap, and `A correction has been filed` passed with no tool call behind
it.

The check also reads the passive form. It anchors on a tracker artifact noun,
which keeps it away from correct passive prose about the game world such as
`The trade was created by a player`.

A passive claim counts as supported when the turn reached the tracker at all,
read or write. Demanding the exact write tool would reject a correct report of
an issue the runtime only looked up. Links are masked first, so the word
`issues` inside a URL path cannot seed a claim.

## When the write fails

`forgejo.issue.failed` carries the failing MCP tool, the HTTP status, and
whether the tool reported the error rather than the transport. The remote
response body is discarded before the error is built, so it never reaches a
log. Without those fields a run of failures gives an operator no path to a
cause.

## See also

See [configuration](sirens-echo-config.md), [MCP tools](sirens-echo-tools.md),
and [the service](sirens-echo.md).
