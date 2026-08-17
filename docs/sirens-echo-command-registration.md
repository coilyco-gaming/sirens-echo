# Publishing the commands

A slash command has three parts: a declaration, a handler, and a registration
with Discord. This repository had the first two and not the third, so every
command it ships was unreachable even with the deployment switch on.

`discordCommands()` rendered the set and its only caller was a test.
`onInteraction` waited for interactions that could not arrive, because Discord
shows a member no command nobody published.

## Per guild, not globally

Registration goes to each guild the access policy admits.

A global registration appears in **every** guild the bot is in, including ones
the policy refuses. Those commands would be visible, invocable, and answered
with `not permitted here`, which advertises a summon path this deployment does
not offer. Per guild keeps the surface and the policy saying the same thing.

A deployment admitting no guild registers nowhere and says so, rather than
falling back to global.

## Bulk overwrite

The write is a bulk overwrite per guild, so the published set is exactly the
declared set. A command removed from `JobCommands()` disappears from Discord
instead of lingering as an invocable ghost whose handler no longer exists.

## Failure is never fatal

A guild that refuses the write is logged as `discord.commands.failed` and the
loop continues, because a partial command surface beats none and the rest of the
service is unaffected. The first error is returned once the loop finishes, so
the failure is reported rather than swallowed.

## When it runs

On `discord.ready`, which is the first point the application id exists. It is
gated on `SIRENS_ECHO_DISCORD_COMMANDS`, unchanged and still defaulting false.

The scope needed is `applications.commands`, which every published install link
now carries. An install predating that correction has to be re-authorised before
a registration can succeed.

## What is still not built

**Promoting an MCP prompt to a command.** The mapping exists in
`promptcommand.go` and the allowlist decided on sirens-echo#127 does not, and
neither does the answer to the harder problem underneath: an interaction must be
answered in three seconds and a model turn takes minutes, so a prompt command
needs either a deferred response or the job path. That is a design decision
rather than a build, and it is tracked on sirens-echo#884.

## See also

- [commands](sirens-echo-commands.md) - the declaration and its schema.
- [access](sirens-echo-access.md) - a slash command is a summon path and passes
  the same gate a mention does.
