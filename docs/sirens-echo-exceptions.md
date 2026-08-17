# Sirens Echo exception taxonomy

SigNoZ groups exceptions by `service.name` and `exception.type`. Echo records
every failure through one typed, source-controlled catalog so grouping and
alert filters remain stable.

## Field contract

- Grouping type - `exception.type` is one of 38 unique `sirens_echo.*` values.
- Human wording - `exception.message` and the span status description use the
  same fixed sentence from the catalog.
- Operational tags - `error.stage` has ten possible values and
  `error.outcome` is a fixed outcome owned by the selected catalog entry.
  `error.fault` is `caller` or `service`, so a caller mistake does not inflate
  the service error rate. The stage cannot stand in for it: `prompt_failed` is
  an MCP failure surfaced on the HTTP path, and `rate_limited` is the service
  refusing a well-formed request. A code declaring neither fails the suite.
- Span projection - `error.type`, `error.stage`, and `error.outcome` repeat the
  same bounded values for trace search and alert filters.
- Stack traces - Echo emits no `exception.stacktrace`, preventing source or
  filesystem paths from entering retained exception data.
- Redaction fallback - an invalid or unclassified code becomes
  `sirens_echo.telemetry.unclassified` without retaining the supplied value.

## Cardinality

Thirty-three types represent operational paths across the `turn`, `history`,
`validation`, `forgejo`, `reply`, `mcp`, `model`, `http`, `jobs`, and
`content_gate` stages. The thirty-fourth is the `telemetry` fallback. This is
the catalog's hard grouping bound. Adding a failure path requires an explicit
catalog entry and a reviewed increase to that bound.

The most recent increase is the two `content_gate` types. A failed classifier
recorded one log line and no span, so a gate that had stopped working was
visible only to somebody already reading that turn's logs. Two rather than one
because a dead classifier and one answering off its own closed list need
different people, and one name would hide the quieter inside the louder. Both
are the service's fault. See
[gate failures](sirens-echo-content-gate-failures.md).

Before that, the five `jobs` types. That surface failed through bare
`http.Error` and recorded nothing, so the failure rate omitted it entirely.
Five rather than fewer because `queue_full` is the service's fault and the rest
are the caller's, and collapsing them would lose the split sirens-echo#159 needs.
See sirens-echo#383.

## Redaction boundary

The recording API accepts only the catalog's numeric code type. Upstream error
text, request identifiers, member data, URLs and paths, credentials, and
payloads cannot be passed into the exception event.

Regression coverage exercises every catalog entry. It checks the event, span
attributes, status description, stack-trace omission, cardinality, and safe
fallback. Representative upstream, request, user, path, credential, and payload
values must not enter retained exception fields.

See [the observability guide](sirens-echo-observability.md) for the surrounding
trace, log, metric, and SigNoZ contracts.
