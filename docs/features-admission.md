# Admission features

Who may reach the service, and when a message becomes a turn. What the service
then does with one is in
[response service features](features-response-service.md).

- Mention-or-reply invocation with channel, thread, guild, author, and
  duplicate gates
- Summoning by an edit that newly names the service, gated on a member edit
  rather than a link preview
- Git-tracked access policy stacking guild, channel, user, and role grants with
  a deny list, per-guild rate overrides, and CI validation
- Per-user, per-context, and global admission control with a bounded queue,
  one cooldown notice per window, and bounded lookups

## See also

See [access](sirens-echo-access.md) for the policy file,
[admission](sirens-echo-admission.md) for the limiter and the queue,
[contexts](sirens-echo-contexts.md) for where a turn may happen,
[summons](sirens-echo-summons.md) for what counts as being named, and
[FEATURES.md](FEATURES.md) for the rest of the inventory.
