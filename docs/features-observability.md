# Observability features

What the service reports about itself. How a turn is admitted, run, and
answered stays in [response service features](features-response-service.md).

- Transport-aware OpenTelemetry ingress and joined turn traces end to end
- Trace-correlated metadata logs with byte counts and no member or model text
- Logs exported over OTLP beside traces and metrics, sharing one `service.name`,
  and kept on stdout so `kubectl logs` survives a SigNoZ outage
- Turn, latency, model-call, tool-call, admission, and failure metrics, plus a
  build-time closed exception catalog tagged by stage, outcome, and fault
- Caller and service faults split on every exception, declared per code so a new
  one cannot be silently unclassified
- A Discord turn span carrying the author, guild, channel, thread, and message
  ids, and no direct message contributing any of them
- A gateway heartbeat counting observed, admitted, and replied, so a quiet
  guild and a stopped ingress differ
- Metrics-only liveness and non-generating route readiness, bounded

## See also

See [observability](sirens-echo-observability.md) for the guided investigation,
[turn identifiers](sirens-echo-turn-identifiers.md) for what a span may carry,
[exceptions](sirens-echo-exceptions.md) for the catalog, and
[FEATURES.md](FEATURES.md) for the rest of the inventory.
