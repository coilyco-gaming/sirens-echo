# Multiple Discord contexts

One deployment can serve several Discord channels across several guilds, plus
direct messages, from a single bot token and a single process. Routing is a
deployment fact, so it lives in environment configuration rather than in the
tracked agent definition.

## Scope configuration

* `DISCORD_CHANNEL_ID` - comma-separated channel IDs that may summon the
  service. A thread whose parent is one of them is in scope too. Channel IDs
  are globally unique, so one list spans guilds without further qualification.
* `DISCORD_GUILD_IDS` - optional guild allowlist. Empty means every guild the
  bot has been added to, still bounded by the channel list. Set it when the bot
  is installed in a guild whose administrators can create channels the operator
  has not reviewed.
* `SIRENS_ECHO_DISCORD_DM_ENABLED` - default `false` - admits direct messages
  sent to the bot. A direct message has no guild moderation behind it, so it
  stays opt-in and adds the direct-message intent only when enabled.

Every ID is validated as a numeric snowflake at startup. A channel name in
place of an ID is a configuration error rather than a service that silently
answers nothing.

## What a context means

A context is the admission identity for everything arriving from the same
origin. All channels in one guild share one context key, so a flood in one
channel spends that guild's budget rather than the deployment's. Each direct
message channel is its own context. The HTTP transport is one context.

## The definition's channel field

`channel` in the tracked definition is the human-readable boundary named in the
system prompt. It is not the routing key, and it no longer has to be `#bots`.
It must be empty or a `#channel-name` that the grounding validator will also
accept, so a configured label can never introduce a reference the model is then
rejected for repeating.

A definition that names no channel is transport-neutral. Its prompt describes
the configured deployment ingress without asserting which one was selected.

## Deploying to a guild the operator does not moderate

1. Add the bot with the narrowest permissions that still work: view channel,
   read message history, send messages, and send messages in threads.
2. Set `DISCORD_CHANNEL_ID` to the exact channels the guild's owner agreed to.
3. Set `DISCORD_GUILD_IDS` so an unrelated guild cannot summon the same
   deployment.
4. Leave `SIRENS_ECHO_DISCORD_DM_ENABLED` unset.
5. Review the admission defaults in
   [admission control](sirens-echo-admission.md) before the first rollout.

The service still answers only a mention or a reply to its own message. It has
no moderation, role, announcement, or account surface, and that exclusion is
independent of how many contexts it serves.

## What a bot cannot do

A bot cannot read or act as a human account. Direct-message support means
members can message the bot, not that the bot reaches into an operator's own
account. A user-installed Discord app receives only application-command
interactions and never message events, so mention-based summoning does not work
in that install mode.

## See also

See [admission control](sirens-echo-admission.md),
[the service walkthrough](sirens-echo.md), and
[configuration](sirens-echo-config.md).
