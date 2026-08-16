# Object emoji in a neutral reply

The neutral profile refused every emoji. It now admits an emoji that names a
thing and still refuses one that carries tone. The reason is legibility: an eye
finds `Wood 🪵` in a wall of text faster than it finds `Wood`. See
sirens-echo#203, where the request is emphatic that this is **not for whimsy**.

## Where the line sits

Refused, and the check enforces it:

* **Emotive** - faces, people, body parts, hands, hearts. These are what make a
  reply read as a person, which is what the neutral profile exists to prevent.
* **Celebration** - fireworks, confetti, sparkles, `💯`. A party popper is an
  object to Unicode and tone to a reader, so the reader wins.
* **Indicators** - status dots, geometric shapes, verdict marks, arrows. `🟢`
  is legible and it is not an object. Kai ruled it out of scope explicitly.

Admitted: everything else, which is items, resources, creatures, places, and
tools. At most **three** in one reply.

## Why a denylist rather than an allowlist

An allowlist of object ranges would refuse any object emoji nobody thought of,
and a style refusal costs a repair attempt and then the turn. A denylist fails
the other way: an unlisted emoji is admitted and reads slightly off. Wrongly
refusing a correct reply is the more expensive mistake, so the check names what
it will not accept.

The model is steered toward objects by
[the object table](../.agents/skills/sirens-echo-knowledge/references/object-emoji.md)
rather than by the check. The check is a floor against tone, not the taste.

## The three that are not the check's job

The doctrine carries these because a validator cannot judge them:

* The emoji follows the object rather than replacing it. `🪵 is listed at 3
  Spectres` is unreadable to a screen reader and to any client that does not
  render it.
* First mention only.
* An object with no obvious emoji is written plainly, because an approximate
  one has to be decoded rather than skimmed.

## The bound, and the risk taken with it

`maxObjectEmoji` is three. Kai declined every numeric bound when density was
first raised, then set three. The earlier design note warned that one emoji per
referenced object across a long list turns legibility back into noise, and three
is the answer to that warning rather than a guess.

A reply carrying a fourth is refused, which costs one repair attempt. That is
deliberate: the doctrine states the bound, so a model reaching four has ignored
an instruction rather than met an edge case.

## What this is not

Harness reactions signalling machine state, `👀 🔨 ❌ 🚫`, are a different
mechanism with a different purpose and must not share this implementation. See
sirens-echo#221.

## See also

* [Response profiles](response-profiles.md) - what the neutral profile promises.
* [Reply repair](sirens-echo-reply-repair.md) - what a style refusal costs.
