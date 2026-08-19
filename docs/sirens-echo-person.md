# The community person package

`agent/compose/person` is an external agent-compose person package holding the roles the community owns
and the Core Roster does not. **Selection is exclusive per request**: a request naming a person-source
composes only that package's roster, so the stage script bakes the Core Roster roles from the embedded
default and each package role from a request that gains one `person-source "person"` node. The package
sits under `agent/compose` because a request's person-source path must be relative and clean, so it
cannot reach a sibling directory. A slug owned by both rosters would silently overwrite a core bundle,
so the script fails on the collision instead.

The package uses agent-compose's sectioned person layout (`roles/`, `personalities/`, `definitions/`),
because the flat `data/` layout documented upstream is wired for the embedded roster alone. The
personalities restate the core bodies verbatim, since agent-compose never merges entities across
packages. The bake compares each restated body against the same
skill as delivered from the embedded roster and fails on a byte difference, so the copies are checked
mirrors rather than silent forks. A package-native personality with no core counterpart is skipped
with a note, which is the trade: drift is caught only while some core role still delivers the body.

`librarian` is the first package role: custodian of the community record, composed for a future fourth
lane. Provenance-first retrieval is the charter: answers read from a named source with the citation
attached, gaps filed rather than papered over, and the record presented beside a conflicting memory
without adjudicating it. It carries no boundary or adjacency entities, because their owners and targets
are not defined roles in a single-role package, and the charter body carries the hand-offs instead. No
lane composes it yet, and `SIRENS_ECHO_ROLE=librarian` selects its baked bundle once one does.

## See also

* [Composing a lane identity](sirens-echo-compose.md) - the allowlist, the request, and the bake.
* [Identity](sirens-echo-identity.md) - role and voice as separate axes.
