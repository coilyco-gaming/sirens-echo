# Shutdown

A restart is not a failure, and a member whose turn it interrupts is entitled to
hear which one it was.

## What a restart used to do

SIGTERM cancelled the context handed to `Run`, which reached the HTTP listener
and nothing else. Every Discord turn was rooted at `context.Background()`, so
shutdown could not see one, wait for one, or tell one to stop. The turn ended
when the process did.

For the member that read as silence. The message kept its accepted mark and
never got an answer, which is the same thing being ignored looks like.

HTTP turns were never affected. They descend from their request, so
`http.Server.Shutdown` already waited for them.

## The drain

Discord turns descend from a root the service owns, and a counter tracks the
ones in flight. Shutdown then goes in one direction:

1. Stop admitting. A summon arriving now gets the refusal mark and no reply,
   because the gateway it would answer through is closing.
2. Wait up to the grace period for the turns already running to answer.
3. Cancel whatever is left, naming the restart as the cause.
4. Give those turns a moment to send their notice, while the gateway is still
   open.
5. Return, at which point `Run` closes the session, the MCP connections, and the
   job runner.

Step 5 was always last. What was missing is everything above it, so the closes
happened while turns were still running.

## Why the cause matters

A cancelled turn sees `context.Canceled`, which is what every other
cancellation also looks like. Only `context.Cause` separates a restart from a
member deleting their message, and the notice depends on that difference:

```
> `service restarting, retry shortly`     the drain ended it
> `turn timed out, retry shortly`         the turn spent its own budget
```

The first is true and actionable. The second would blame the member's question
for a deploy.

The failure metric splits the same way, on `shutdown` rather than
`stage_failed`, so a rollout does not read as an outage.

## The grace period

`SIRENS_ECHO_SHUTDOWN_GRACE`, 15 seconds by default. It has to fit inside the
pod's kill window, and no manifest sets `terminationGracePeriodSeconds`, so
Kubernetes' default of 30 seconds is the ceiling.

It is deliberately shorter than `RequestTimeout`, which is 3 minutes. A turn
allowed to run that long cannot be waited out by any value that fits the
window, so the grace serves the common turn and cancels the rare one.

## See also

- [Notices](sirens-echo-notices.md) - the harness words, and why they take this
  shape.
- [Reactions](sirens-echo-reactions.md) - the refusal mark a drained summon
  gets.
