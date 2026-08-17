# Contexts and threads

Where a turn arrives from, and where a long answer goes.

## Multiple Discord contexts

One deployment can serve several Discord channels across several guilds, plus direct messages, from a
single bot token and process. **Routing is a deployment fact**, so it lives in environment configuration
rather than the tracked agent definition. The full form is the git-tracked
[access policy](sirens-echo-access.md); without it, `DISCORD_CHANNEL_ID` lists the channels that may
summon the service plus their threads (**globally unique, so one list spans guilds**),
`DISCORD_GUILD_IDS` is an optional guild allowlist, and `SIRENS_ECHO_DISCORD_DM_ENABLED` defaults
`false`. Every ID is validated as a numeric snowflake at startup, so **a channel name in place of an ID
is a configuration error rather than a service that silently answers nothing**.

A context is the admission identity for everything arriving from the same origin. **All channels in one
guild share one context key**, so a flood in one channel spends that guild's budget rather than the
deployment's. Each direct-message channel is its own context, and HTTP is one context.

`channel` in the tracked definition is the human-readable boundary named in the system prompt, **not the
routing key**, and no longer has to be `#bots`. It must be empty or a `#channel-name` the grounding
validator will also accept, **so a configured label can never introduce a reference the model is then
rejected for repeating**.

Deploying to a guild the operator does not moderate: add the bot with the narrowest permissions that
still work (view channel, read message history, send messages, send messages in threads), name the exact
channels the guild's owner agreed to, grant members by role rather than by listing accounts, consider a
tighter `rate_limit` than the deployment default, and leave direct messages off. **In a guild the
service answers only a mention or a reply to its own message**, with no moderation, role, announcement,
or account surface.

**A direct message needs no mention**, because it is addressed to the service by definition and the
mention gate exists to keep a busy channel quiet. So every direct message from an allowlisted account
spends a turn, with the per-user admission budget the only remaining brake. **A bot cannot read or act
as a human account**, and a user-installed Discord app receives only application-command interactions
and never message events, so mention-based summoning does not work in that install mode.

## When a reply gets a thread

Two conditions, both required: the turn posted a progress line, so something in the channel points at
the thread, and the turn ran past `turnLongReplyAfter`, the wait plus two beats. **The second alone is
not enough**, because a turn that never posted a line has nothing referring to a thread and a member
would be left with a question and no visible answer. The progress line **is** the announcement, which
also makes the guild's hide-after setting cheap: a thread that auto-hides takes nothing with it that was
not already in the channel.

The title summarises the member's intent, so *how much does it cost to build a log house* becomes
something like *log house pricing*. An earlier version refused to summarise, on the grounds that
summarising is writing a member a title; **that was overruled deliberately in issue 461**. The summary
needs the model, so it is one short extra completion, proportionate because a thread only happens on a
turn that already ran past the long-reply window. **It degrades rather than fails**: if the call errors,
times out, or returns nothing usable, the thread is still created with the mechanically derived name.
The summary takes the same cleaning as a derived name, so it cannot introduce markup a member's own
message could not.

**A thread that cannot be made must not cost a member their reply.** No permission, a channel type that
cannot hold threads, an API failure, a turn already inside a thread: every one returns "no thread"
rather than an error, and the reply goes to the channel as before. That is why the decision returns a
channel id and a boolean rather than an error: **no failure here is worth failing a turn over**. The
nesting check reads cached gateway state and makes no API call. Nothing opens a thread for a job, so a
job started in a channel has no referent of its own.

## Thread title length

**Fifty characters, and an over-long one is asked again rather than cut.** `threadTitleRunes` is 50.
Discord's own cap is 100 and `threadNameRunes` still holds it, but 100 does not display whole in the
thread list or in the narrow surfaces that truncate hardest (Kai's decision on sirens-echo#753). A
trimmed title loses its subject, the one thing a title exists to carry, so an over-length title goes
back to the model with the limit stated. `threadTitleRetryPrompt` builds that sentence from
`threadTitleRunes`, **so the number in the prose cannot drift from the number in the check**.

**Exactly one regeneration.** If the second answer is also over, the title is hard-trimmed and recorded
as `thread.title.trimmed`, never a loop, because **a title generator must not be able to spend a turn's
budget on itself**. The trim is a plain cut rather than `truncateRunes`, which spends a rune on an
ellipsis saying it truncated. Recording it matters as much as the trim, since a generator that keeps
overrunning is a prompt problem a silent fallback would hide.

**The bound holds at creation, not only in the generator.** `threadCreationName` bounds whatever it is
handed: the generated title is one source, the derived name from the member's own message is the other,
and it was previously free to reach 100. This binds creation only, so nothing renames an existing
thread.

## Reading a whole thread

A turn inside a thread got the same partial window a channel turn gets, **so a thread longer than that
window was answered from its own tail** with nothing saying so. The design pass specified a per-channel
toggle defaulting off, and Kai removed it on sirens-echo#769: a turn inside a thread reads the whole
thread, everywhere. `SIRENS_ECHO_THREAD_PREFILL_CHANNELS` is retired and **inert rather than a boot
failure**, so a stale value cannot take the service down. Removing the toggle also removed the staged
rollout the spec assumed, so the bounds below carry the whole context risk on their own.

**When the thread exceeds the context budget the oldest messages drop until it fits, and the reply says
so.** Kai chose that over silent truncation (a wrong answer from missing context would look identical to
any other), falling back to the partial window (the feature would quietly stop applying in exactly the
long threads it was built for), and summarising the older half (a model call and a new failure mode on
every turn). The annotation is a service-authored suffix, so it contends for the send budget, and **it
sits ahead of the tool receipt in the preference order**: a note about missing context outranks a record
of what ran, because the last suffix is the first one cut.

The walk is bounded, so a pathological thread costs a known number of Discord calls. A thread longer
than the walk is still truncated and annotated, and the annotation says `at least` before the length,
because at that point the runtime knows a floor rather than the thread. **An absent hedge is therefore a
claim**: a plain count means the walk reached the start. A thread whose newest messages all fit is still
annotated when the walk stopped short, because nothing going over budget is not the same as having read
the whole thread.

A whole-thread read is one Discord call per hundred messages instead of one, on every thread turn. Read
`history.thread.read` and `history.thread.dropped` on the `community.history` span for what real threads
produce, and note that sirens-echo#750 raises the number of turns taken inside threads. Outside a
thread the prefill is the same partial window, and a turn that drops nothing adds nothing to the reply.
