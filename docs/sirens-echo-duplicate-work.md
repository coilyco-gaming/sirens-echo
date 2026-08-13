# Duplicate work

A claim reserves an issue number. The collision surface is files, and two issue
numbers can point at one function, so claiming correctly does not stop two
agents building the same change. See sirens-echo#552 and sirens-echo#690.

## Measured, one seat, ninety minutes

```
#619 fix built        -> #623 merged the same fix three minutes in
#627 test built       -> #634 merged a superset
#628 validator built  -> #639 merged it during the gate run
#663 wildcard built   -> already merged at push
#684 roster fix built -> #685 merged it during the gate run
#682 doc written      -> the same filename was already on main
```

Every one was claimed on its issue first. No claim was jumped.

## The gap is two to five minutes

In each case the competing change merged **after** the branch was cut and
**before** the push. So the guard is not a better claim, it is a fresher read.

**Fetch `origin/main` immediately before the first edit.** Branching is not
close enough when the base moves several times an hour.

**Look for the thing before building it.** Grep for the function, the file, the
flag. One of the six above was a document whose filename already existed.

## When you are beaten to it

**Compare, then discard.** Three of the six produced something worth landing
once the landed version was read properly rather than argued against:

- a fetch matcher admitted a host with an empty first label, which the merged
  version did not guard
- a notice deadline had no test, which the merged version did not add
- a rule page recorded a seam that the shipped tool does not use

The rest were dropped, including one pull request of my own that was a strict
subset of what landed.

**The landed version is often better.** It arrived first for the same reason it
was reviewed first, and reopening a settled question to keep your own version
costs more than the duplicate did.

## What this does not fix

A genuine race. Two agents starting inside the same minute still collide, and
no habit closes that. That half stays on sirens-echo#552, which carries the
coordination question and needs a decision rather than a habit.

See [the consult gate](sirens-echo-consult-gate.md) for the other pair of
habits that attach to something you are already doing.
