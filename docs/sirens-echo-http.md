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

## Authentication and limits

`SIRENS_ECHO_HTTP_TOKEN` is required whenever the listener is not bound to
loopback, which the k3s deployment's `0.0.0.0` bind always is. Startup fails
rather than serving an unauthenticated completion endpoint. The token is
compared in constant time, and neither health route requires it.

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
  -H "Authorization: Bearer ${SIRENS_ECHO_HTTP_TOKEN}" \
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
