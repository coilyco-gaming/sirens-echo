# What this process can do

A capability that was never switched on is indistinguishable, from inside the
process, from one that was never built. That is a consequence of a property
worth keeping: an unset variable offers no tools rather than tools that fail.

The cost is that establishing "is feature X actually on?" meant reading the
harness and the deployment together, by someone who already suspected the
answer. Three capabilities were found built and inert that way, each by
accident. See sirens-echo#539.

## One line, at boot

`Run` emits `capabilities` once, before the gateway opens. It names the content
gate, the job store kind, jobs, the scratchpad, fetch, the issue tracker, the
Discord surfaces, and the roster size.

## Presence, never values

The line says whether a capability is on. It does not say where the scratchpad
is mounted, which hosts fetch may reach, or which channel is served. A log line
naming those grows into an identifier surface, which this service guards
against elsewhere, and the operator asking "is it on" does not need them.

The roster is a count for the same reason.

## It reports, it does not judge

Nothing here fails a startup. The runtime cannot tell whether an inert
capability is a mistake, because a profile that legitimately has no scratchpad
and one that was meant to have it are the same process. Only deployment knows
which, so the runtime states the fact and leaves the judgement.

The job store is the sharpest case and is a string rather than a bool: the
durable store and the one that drops every in-flight job on a roll are both
"a job store" to a reader who only learns that one exists.

## See also

- [capability limits](sirens-echo-capability-limits.md) - what the service
  refuses to do, as opposed to what it is configured to do.
