# Structured commands

Free text is the right interface for conversation and the wrong one for
consequential actions. "Deploy the thing" and "deploy the other thing" are one
paraphrase apart, and a natural-language turn gives the harness no structural
way to tell them apart before acting.

Commands add a structured path. They do not replace the conversational one.

## The schema is an authority boundary

Two deploy-repository issues failed the same way: the surrounding tooling bounds
tool *names* and not tool *arguments*. Commands are the first place this harness
owns an argument schema itself rather than inheriting one, so the schema is
treated as authority rather than as input validation.

What that means concretely:

* An **undeclared argument is refused**, never ignored. Ignoring one lets a
  caller believe it took effect.
* Every string carries a **length bound**, defaulted rather than optional.
* A value that names a thing carries **choices**, which is the tightest bound
  available. A value that cannot be a fixed set carries a **pattern**.
* A command **submits exactly one job kind**, and that kind must be declared, so
  the command surface cannot outgrow the job surface.
* The declaration is **validated at startup**, so a malformed command fails
  before a caller ever reaches it.

The same declaration renders Discord's options and binds the incoming
arguments, so the advertised schema and the enforced one cannot drift.

## A command is a summon path

Discord routes an interaction to the bot because the user picked it, not because
they mentioned anyone. That bypasses the mention gate by construction, which is
the concern raised in #127.

So a command passes the gates a message passes, in the same order: the access
policy first, then admission. A command reaches nothing a message from the same
caller in the same place could not.

Registration is a **write** to Discord's API and needs the
`applications.commands` scope, which widens what the token does. It is therefore
off unless `SIRENS_ECHO_DISCORD_COMMANDS` says otherwise, and the deployment
opts in deliberately rather than inheriting it.

Every path answers with a notice, so a command never ends in silence.

## Thread binding

Once work is durable, a follow-up like "cancel that" needs a referent. The
thread is the natural one.

The binding lives **on the job record**, as `Origin.ThreadID`. Resolving is a
lookup of a recorded fact, never an inference from recent history, and the
record is the single source of truth rather than a second index that can drift.

It is **singular in both directions**. A thread cannot be bound to a second job,
and a job cannot be repointed at another thread. Rebinding the same pair is
idempotent, which keeps a retry safe.

`job-status` and `job-cancel` take an optional job id and fall back to the
binding, so a follow-up inside the thread repeats nothing. An explicit id always
wins. Outside a bound thread with no id, the command says it has no referent
rather than guessing.

**A job binds to the thread it was started in**, and only to a thread. Binding a
channel would make it resolve to one arbitrary job of many. A thread already
bound keeps its first job, so a second started there has no referent, and
nothing opens a thread for a job, so one started in a channel has none.

## What a command does not do

It does not carry per-requester authority. Every job runs under pod-level
authority regardless of who asked, which is stated in #145 and remains true
here. The principal is recorded, and recording is not granting.

See [the lifecycle](sirens-echo-jobs-lifecycle.md),
[access](sirens-echo-access.md), and [/mcps](sirens-echo-mcps-command.md).
