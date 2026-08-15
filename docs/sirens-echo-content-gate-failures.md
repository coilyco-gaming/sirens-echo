# Content gate failures

How a broken classifier reports itself. Why a broken gate is not a denial is in
[the content gate](sirens-echo-content-gate.md).

## The kind is in the slug

A failed gate emits its own span under the turn, named for how it broke:

- `content.gate.failed.model` - the classifier call itself failed
- `content.gate.failed.unknown_class` - the call returned and named a class
  outside the closed list

Before this the failure was one log line carrying a raw error string, and no
span at all. A gate that had stopped working was visible only to somebody
already reading logs for that turn.

## Why the two are separate

They need different people. A dead classifier is a dependency to chase. One
answering off its own list is a prompt or a taxonomy that has drifted, on a
model that is otherwise healthy and answering everything else.

Under one name they read as the same outage, and the quieter one hides inside
the louder whenever they fail at different rates. That is the ordinary case
rather than the unusual one.

Both are the service's fault, the second included. A model answering outside
its own closed list is not something a member did, so neither inflates the
caller-fault rate. See [the exception taxonomy](sirens-echo-exceptions.md).

## The suffix is closed

Grep the `content.gate.failed` prefix for every failure and the full name for
one kind.

The suffix is a fixed vocabulary and never model output. The class the
classifier invented is exactly the tempting thing to put in the name, and it
would make the name unbounded: one span name per invented string, which is a
grouping key that never groups. It goes to the log line beside the span, so it
stays recoverable, and the span stays clean of it.

## What the turn span still says

`content.classified` is false for a failed gate exactly as it is for one that
never ran. From the turn's side those are the same fact, and the failure span
is the thing that tells them apart.

That tag is unchanged on purpose. Making it three-valued would push the
distinction into every consumer of a boolean that reads correctly today.

## See also

- [the content gate](sirens-echo-content-gate.md) - the gate itself.
- [the exception taxonomy](sirens-echo-exceptions.md) - the catalog the failure
  span records through.
