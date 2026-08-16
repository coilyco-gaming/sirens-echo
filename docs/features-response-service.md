# Response service features

What the Discord and HTTP response surface ships today. What the service
reports about itself is in [observability features](features-observability.md),
who may reach it is in [admission features](features-admission.md), and the
rest of the inventory stays in [FEATURES.md](FEATURES.md).

- Deploy-selected verified role bundle for Deep, none for Echo
- Neutral Sirens Echo and social CoilyCo profiles with independent policy roots,
  a Kai-only trust boundary, and build-time checks
- Definition-selected history budget, skill roots, MCP roster, issue tracker
- Serialized turns with bounded Discord-history continuity
- Agent Proxy loop for MCP schemas, tool calls, results, and continuation
- Impersonal response contract rejecting greetings, emotive emoji, banter, sign-offs,
  and open-ended offers
- Public Eco MCP for current game information
- Private Forgejo MCP fixed to this repository with issue create, close,
  comment, label changes, and bounded reads
- No Forgejo token in the Echo pod, one guarded handler serving the model's
  issue tools
- Plain-text replies with one style-aware repair, bounds, grounding checks, and
  neutral-style validation where selected
- Grounding checks reading first-person and passive claims alike, over prose
  with links masked out
- Appended canonical links for every issue a turn observed or filed, built only
  from returned tool results and bounded by the send budget
- Reply refusal for any identifier the process holds, derived from configuration
  at boot, admitted by shape, and matched by value rather than spelling
- Caller-supplied history marked as asserted rather than observed, on both the
  HTTP route and the MCP turn tool
- Harness reactions for acceptance, a tool round, a failure, and a refusal,
  applied before the model call and unable to fail a turn
- A worklog element on a long Discord turn, one row per tool call resolving in
  place, degrading to stacked notice lines where the embed permission is
  absent. See [the worklog](sirens-echo-worklog.md)
- Oversized tool results saved to the requester's scratchpad instead of being
  truncated away, with every failure falling back to truncation
- Replies over the Discord send budget attached whole as a file, with the
  message naming it and its size. See
  [reply overflow](sirens-echo-reply-overflow.md)
- Whole-thread prefill for every turn inside a thread, dropping oldest first
  with the loss stated in the reply. See [thread
  prefill](sirens-echo-thread-prefill.md)
- Soft-reference replies with every Discord mention disabled
- One assembly step for every service-authored suffix, shortening the answer so
  no suffix is budgeted against another. See
  [reply assembly](sirens-echo-reply-assembly.md)
- Repo-local Sirens knowledge with no automatic memory or autonomous edits
- One org-relationship source both profiles compose. See
  [organizations](sirens-echo-organizations.md)
- Guarded Discord context skill with SSM fast paths and bounded MCP reads
- Model-filed ordinary Forgejo issues through the guarded tool, with
  exact-title reuse and no labels
- Private HTTP entrypoint over the same turn path, served as JSON and as an MCP
  tool, with W3C tracing and Discord's admission policy
- Transport-neutral CoilyCo profile with no assumed domain, MCP, automatic
  issue tracking, or default write surface
- An undelivered reply reported to the member once and never retried
- Uploaded text stored in the requester's scratchpad, from a Discord CDN
  address only and decided by its bytes
- Offline harness, MCP fixtures, a non-mutating gate, and a graded board

## See also

See [the service](sirens-echo.md), [profiles](response-profiles.md),
[MCP tools](sirens-echo-tools.md), [access](sirens-echo-access.md),
[admission](sirens-echo-admission.md), [contexts](sirens-echo-contexts.md),
[summons](sirens-echo-summons.md), [HTTP](sirens-echo-http.md), and
[rollout](sirens-echo-rollout.md).
