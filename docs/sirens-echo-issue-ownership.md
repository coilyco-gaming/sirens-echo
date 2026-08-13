# Sirens Echo issue ownership

What Echo can do to the tracker is fixed by deploy, not chosen by the model. The
tool loop that reaches it is [runtime MCP tools](sirens-echo-tools.md).

## The published surface

Deploy fixes the Sirens Echo Forgejo MCP guardfile to
`coilyco-gaming/sirens-echo`. The published surface includes issue and label
reads, issue creation and closing, comment creation, and label add, replace,
and remove. There is no owner or repository argument to redirect. There is no
issue-body edit, comment edit, delete, reopen, pin, release, pull-request,
repository, organization, or account tool.

The bound is the guardfile rather than the prompt. A model that decided to
write somewhere else has no verb that accepts a destination, so the boundary
does not depend on the model agreeing with it.

## What naming the tracker changes

Naming that server as `issue_tracker` selects the prompt's issue-filing policy,
which tells the model to search by title and then file a sanitized unlabeled
issue through the MCP tool it already holds. There is no worker-side path and no
Forgejo credential in Echo.

The CoilyCo definition names no tracker and gets the plain uncertainty
instruction, so the two differ in what the prompt asks for and not in what the
model can reach. Both hold the same repository-fixed server. Only one is told
that filing is a thing it should do.

That distinction is worth keeping straight when reading a definition: the
roster answers what is reachable, and the tracker name answers what is
encouraged.

## Ceilings

Connecting, listing, and calling each carry their own ceiling. See [the
budget](sirens-echo-budget.md).

## See also

- [runtime MCP tools](sirens-echo-tools.md) - the loop and its validation.
- [notices](sirens-echo-notices.md) - what a failure tells the member.
