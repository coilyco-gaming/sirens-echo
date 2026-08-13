# What a content block says

A block is the reply a member sees while probing a boundary. That makes it the
worst possible place for unguarded output, and the place where saying less is
worth more than saying it well.

## The channel this closes

Every reply guard runs on the reply produced by the reply turn: the identifier
guard, the identity check, the response style, and grounding. A content verdict
comes from a different turn and short-circuits that path, so its reason would
reach a member having passed none of them.

That reason is model text. Left unbounded it could claim to be a person, echo
the operator's handle or user ID, or run to three sentences, and it appears
exactly when someone is testing what the service will do. The bounds therefore
exist before anything can emit through them, rather than being added after the
classifier that produces them.

## A sensitive class carries no model text at all

For a class marked sensitive the whole design is naming no category, because
naming it tells a member what to avoid saying next time. A model-written reason
there is redundant at best and leaks the hidden signal at worst, so a sensitive
block is a fixed redirect and nothing else.

## An ordinary block may explain itself, briefly

A denied class that is not sensitive keeps its reason when the reason survives
every check the reply path applies, plus two of its own:

- a word cap, because every volunteered justification is a handle to pull and
  this reply only ever appears at a boundary
- one line, because a multi-line refusal reads as an argument and an argument
  invites the next message

## Failing safe means still refusing

Any reason that is empty, too long, first person, claiming to be human, or
carrying an identifier falls back to the fixed redirect. It does not fall back
to answering. A block that fails open is the worst outcome available, so every
path out of this function is still a refusal.

## Not the notice constructor

`harnessNotice` renders one short technical phrase and strips everything outside
a narrow alphabet, silently. It would remove a mark, flatten a structured block
to a single line, and still match the notice shape, so tests would pass on a
mangled result. A block is a different object and has its own constructor.

## See also

* [Content classes](sirens-echo-content-classes.md) - the taxonomy and what
  sensitive means.
* [Notices](sirens-echo-notices.md) - the harness phrase this deliberately is not.
* [Identity](sirens-echo-identity.md) - what any reply may not claim.
