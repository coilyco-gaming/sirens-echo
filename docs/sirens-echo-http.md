# Private HTTP entrypoint

The same `runTurn` path Discord uses is available through `POST /v1/turn` on the
process's private HTTP listener. A deployment can disable Discord and use this
as its only ingress.

## Contract

The endpoint accepts a JSON object with `author`, `content`, an optional
`request_id`, and optional bounded `history`. It returns the validated reply
without sending a Discord message.

It bypasses only Discord's channel, mention, and duplicate gates. Admission,
Agent Proxy, MCP tool calls, response validation, grounding, and guarded Forgejo
issue handling are unchanged.

## Echo as an MCP server

The same turn is served over MCP at `/mcp` on the same listener, as a single
`turn` tool taking `author`, `content`, and optional `history`. A fleet client
reaches Echo natively instead of learning this JSON contract, and `/v1/turn` is
unchanged for existing callers.

Nothing is bypassed. The tool runs the same admission, serialization, response
validation, and grounding as every other ingress, and its turns are labelled
`mcp` in telemetry. Admission keys off `X-Sirens-Caller` when a client sends one
and the declared MCP client name otherwise, so a client can still separate its
own callers. A caller-fixable problem comes back as an error result rather than
a protocol error, so the calling model can see it and correct itself.

Rostering Echo into its own roster is not guarded against. Turns are serialized,
so a self-call waits on the slot its caller already holds and fails on the queue
timeout rather than recursing.

## Access and limits

Reachability is decided at the network layer rather than by the process. The k3s
deployment binds `0.0.0.0:8080`, publishes only a private ClusterIP Service with
no Ingress, certificate, DNS record, or NodePort, and routes callers through
Echo's Tailscale sidecar, so reaching `/v1/turn` requires being an authorized
node on the tailnet. The process carries no credential of its own. A deployment
that exposes the listener any other way owns that boundary itself.

`X-Sirens-Caller` selects the per-caller admission budget. Anonymous clients
share one budget. A limited caller receives `429` with `Retry-After`. See
[admission control](sirens-echo-admission.md).

## Usage

The process binds to `127.0.0.1:8080` by default. The k3s deployment sets
`SIRENS_ECHO_HTTP_ADDR=0.0.0.0:8080` and exposes it only through Echo's
Tailscale sidecar. From an authorized tailnet client:

```sh
curl -sS http://sirens-echo:8080/v1/turn \
  -H 'Content-Type: application/json' \
  -H 'X-Sirens-Caller: manual-test' \
  -d '{"author":"manual test","content":"What is the current Eco server status?"}'
```

## Health and tracing

`GET /healthz` checks local listener liveness and `GET /readyz` checks the
configured route through Agent Proxy without inference. Both emit bounded
metrics and no logs or spans. See the [health contract](sirens-echo-health.md).

The deploy bundle adds a private ClusterIP Service but no public Ingress,
certificate, DNS record, or NodePort. `POST /v1/turn` remains traced, and its
server span extracts W3C context and parents the shared Community turn.

## See also

See [the service](sirens-echo.md), [admission](sirens-echo-admission.md), and
[configuration](sirens-echo-config.md).
