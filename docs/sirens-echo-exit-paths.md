# Sirens Echo exit paths

The entrypoint's own failures are JSON on stdout carrying a populated `level`,
in the same shape as every other record, so ingest promotes a severity from
them and a severity alert can match the process dying.

## The records

| Message | Emitter | Exits |
| --- | --- | --- |
| `startup.config.failed` | startup logger | yes |
| `startup.telemetry.failed` | startup logger | yes |
| `startup.agent.failed` | `Telemetry.Error` | yes |
| `run.failed` | `Telemetry.Error` | yes |
| `shutdown.telemetry.failed` | `Telemetry.Error` | no |

Each carries the cause under `error`. The first two land before `Telemetry`
exists, so they use a standalone startup logger with the same handler
configuration. The rest are trace-correlated like every other error.

## Why the stdlib log package must not reach this binary

`log.Fatalf` writes unstructured text to stderr. The ingest pipeline parses a
JSON body, so an unstructured line fails the parse, produces no `level`
attribute, and reaches neither the severity parser nor an alert filtering on
`level`.

That makes a crash the one event no severity alert can see, which is the
opposite of what a fatal path is for. Route new failures through the startup
logger before `Telemetry` exists and through `Telemetry.Error` after.

## Known gap

A fatal path exits without running the deferred telemetry shutdown, so a crash
does not flush its pending traces and metrics. The log record still reaches
stdout, because the handler writes synchronously. This is unchanged from the
stdlib `log.Fatalf` behaviour it replaced and is recorded here rather than
fixed.

## See also

- [observability](sirens-echo-observability.md) - the full telemetry contract.
- [exception taxonomy](sirens-echo-exceptions.md) - handled failure grouping.
