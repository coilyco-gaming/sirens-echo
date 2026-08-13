# The issue tracker surface

What the Sirens Echo Forgejo MCP publishes, and what naming it `issue_tracker`
changes. The tool loop that reaches it is in
[runtime MCP tools](sirens-echo-tools.md).

## Fixed to one repository

Deploy fixes the Sirens Echo Forgejo MCP guardfile to
`coilyco-gaming/sirens-echo`. The published surface includes issue and label
reads, issue creation and closing, comment creation, and label add, replace,
and remove. There is no owner or repository argument to redirect. There is no
issue-body edit, comment edit, delete, reopen, pin, release, pull-request,
repository, organization, or account tool.

Naming that server as `issue_tracker` selects the prompt's issue-filing policy,
which tells the model to search by title and then file a sanitized unlabeled
issue through the MCP tool it already holds. There is no worker-side path and no
Forgejo credential in Echo. The CoilyCo definition names no tracker and gets the
plain uncertainty instruction, so the two differ in what the prompt asks for and
not in what the model can reach.

Connecting, listing, and calling each carry their own ceiling. See [the
budget](sirens-echo-budget.md).

## See also

- [knowledge gaps and corrections](sirens-echo-issues.md) - when a turn files
  one of these, as opposed to what it may reach.
- [runtime MCP tools](sirens-echo-tools.md) - discovery, validation, and the
  loop this server is called through.
