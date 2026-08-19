# Boundaries a lane has no seat for

Why Dowel composes without the boundaries its role defers.


A defer-side boundary routes work to the role that owns it, so `engineer` deferring
`modify-live-system` means hand it to DevOps. **Every lane here is one agent alone in a guild**, so
there is no DevOps to hand it to and the rule reads as a stop rather than a handoff. Dowel runs
`engineer`, and the stage script emits `boundary-omit` for that role so both of its defer-side
boundaries leave the bundle entirely: no body, no name on the identity card, no manifest entry
([agent-compose#304](https://forgejo.coilysiren.me/coilyco-flight-deck/agent-compose/issues/304)).

The omission is keyed on the **role**, because one request template bakes all eight and the role slug
is the only key the bake has, the same constraint that makes `seat_identity` key on `ops`. Echo and
Deep keep every boundary they compose today, including the ones they own, which cannot be omitted at
all. What replaces the removed doctrine is lane-local: `tooling-sirens-dowel-contract` states the
deployment limit on its own terms rather than as a qualifier on a boundary that is no longer there.
