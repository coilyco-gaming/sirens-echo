# Sweep run protocol

A multi-model sweep compares behavior across model tiers. It is only a
comparison if the substrate was equal, so this protocol governs how a cell is
run and when its result is thrown away.

## The failure this exists to prevent

The local GPU tier becomes unusable when the host is doing anything else, and
nothing in the path detects it, routes around it, or reports it. A starved
backend produces timeouts, truncated generations, and degraded output.

End-state scoring cannot tell that from a model that genuinely cannot do the
task. Run the self-hosted cell on a contended host and the sweep concludes that
the open-source tier cannot do the behavior under test. That is the finding the
sweep was designed to look for, arrived at for the wrong reason, and it is
believable because it matches the prior. Data-shaped wrongness is worse than no
data.

## Rules

Run the self-hosted cell only against a host known to be idle. Verify before
the cell rather than after, because after is an alibi rather than a control.

Record host state for every cell, in the run's own provenance, at the time it
ran. Not in a separate note, and not from memory afterwards.

A cell that hits a saturated backend or a deadline is **void**. Re-run it. It is
not a fail, and scoring a starved backend as a model failure is the whole
mistake this protocol prevents.

Never compare a cell to another cell taken under different host conditions.
Re-run both rather than reasoning about the difference.

## What the runner can and cannot do for you

The rate runner separates an `error` outcome from a `fail` outcome, excludes
errors from the denominator, and reports a case whose attempts all errored as
`measured: false` rather than as a pass. That covers the substrate failing
loudly.

It cannot cover the case that matters most. A contended GPU that returns a slow
but complete reply raises no error. The runner sees a reply, scores it, and
records a behavioral failure, and from inside the turn that is indistinguishable
from a model getting it wrong. No in-process check closes this, which is why the
host-state record is the load-bearing rule rather than the void rule.

## Recording provenance

`SIRENS_ECHO_SUBSTRATE` and `SIRENS_ECHO_IMAGE` are copied verbatim into the
emitted provenance. Give the first a host and its state, for example
`kai-tower-3026, GPU idle, verified before run`. Give the second the build the
measured service is running.

The image matters more than it looks. Roughly half of main's pushes publish no
image, so the deployed build is often older than the change under measurement,
and a dataset that does not name it still looks entirely legitimate.

Unset, either field records `unrecorded`. That is deliberate and not a neutral
default: a dataset that does not know its host or its build should say so where
a later reader cannot miss it. Treat `unrecorded` as unusable for any
comparison.

## Scope

This governs sweeps and rate runs. The deterministic gate is unaffected: it runs
once per deployment and a starved backend fails it loudly rather than quietly
producing a wrong number.
