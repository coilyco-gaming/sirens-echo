# Where a log line goes

Every line goes to two places: stdout, and the SigNoZ collector over OTLP/HTTP.
The exported copy carries the same resource the traces do, so `service.name` is
the same value on both signals.

## What it replaced

The harness exported traces and metrics and no logs. Lines still reached
SigNoZ, because the cluster agent scrapes pod stdout off disk, but a scraped
row cannot carry `service.name` - the scraper has no way to know it. Those rows
were keyed by `k8s.deployment.name` instead.

That mostly worked, and three things stayed awkward. `service.name` is the key
every SigNoZ doc and the MCP's own shortcut reaches for, and on those rows it
matched nothing and returned an empty result rather than an error. Traces and
logs were keyed differently, so no dashboard or alert could use one field
across both. And the two lanes share an image, so `k8s.container.name` is
`sirens-echo` in both namespaces and only the deployment name separated them.

## Why stdout stays

Emitting both means every line is stored twice, once scraped and once over
OTLP, with different attribute sets on the two rows. That cost is accepted
deliberately.

Dropping stdout would make `kubectl logs` useless for these pods, and that is
the path that works when SigNoZ is itself the thing that is unreachable -
which is exactly the incident where the logs matter most. A duplicated line is
cheaper than a blind incident.

The duplication is the part a future reader cannot infer from the code, so it
is written here rather than left as an apparent oversight.

## The fan-out

`slog` has no fan-out, so `multiHandler` writes one record to every handler.
Three properties matter and each has a test:

* A failing exporter does not stop the line reaching stdout. Its error is
  reported, not swallowed, and the other destination still writes.
* A destination that wants a level gets it even when another does not, so
  raising one threshold cannot silence the other.
* Deriving a logger with `WithAttrs` or `WithGroup` does not reshape the
  handler it came from, so one logger's attributes cannot leak into another's.

Records are cloned per destination, because a handler may retain or modify what
it is given and the next one must see the record as it arrived.

## What did not change

The lines themselves, their level, and what they may carry. Tokens, prompts,
tool payloads, model bodies, and replies are still never logged. Adding a
second destination moves no content into telemetry that was not already in it.

## See also

* [Observability](sirens-echo-observability.md) - what is logged, and the
  metric and trace instruments beside it.
