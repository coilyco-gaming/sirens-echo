# Response service features

What the Discord and HTTP response surface ships today. The rest of the
inventory stays in [FEATURES.md](FEATURES.md).

- Deploy-selected verified role bundle for Deep, none for Echo
- Neutral Sirens Echo and social CoilyCo profiles with independent policy roots,
  a Kai-only trust boundary, and build-time checks
- Definition-selected history budget, skill roots, MCP roster, issue tracker
- Mention-or-reply invocation with channel, thread, guild, author, and
  duplicate gates
- Summoning by an edit that newly names the service, gated on a member edit
  rather than a link preview
- Git-tracked access policy stacking guild, channel, user, and role grants with
  a deny list, per-guild rate overrides, and CI validation
- Per-user, per-context, and global admission control with a bounded queue,
  one cooldown notice per window, and bounded lookups
- Serialized turns with bounded Discord-history continuity
- Agent Proxy loop for MCP schemas, tool calls, results, and continuation
- Impersonal response contract rejecting greetings, emojis, banter, sign-offs,
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
- Soft-reference replies with every Discord mention disabled
- Repo-local Sirens knowledge with no automatic memory or autonomous edits
- Guarded Discord context skill with SSM fast paths and bounded MCP reads
- Model-filed ordinary Forgejo issues through the guarded tool, with
  exact-title reuse and no labels
- Transport-aware OpenTelemetry ingress and joined turn traces end to end
- Trace-correlated metadata logs with byte counts and no member or model text
- Turn, latency, model-call, tool-call, admission, and failure metrics, plus 23
  bounded SigNoZ exception groups with stage and outcome tags
- Metrics-only liveness and non-generating route readiness, bounded
- Private HTTP entrypoint over the same turn path, served as JSON and as an MCP
  tool, with W3C tracing and Discord's admission policy
- Transport-neutral CoilyCo profile with no assumed domain, MCP, automatic
  issue tracking, or default write surface
- Offline harness, MCP fixtures, a non-mutating gate, and a graded board

## See also

See [the service](sirens-echo.md), [profiles](response-profiles.md),
[MCP tools](sirens-echo-tools.md), [access](sirens-echo-access.md),
[admission](sirens-echo-admission.md), [contexts](sirens-echo-contexts.md),
[summons](sirens-echo-summons.md), [HTTP](sirens-echo-http.md), and
[rollout](sirens-echo-rollout.md).
