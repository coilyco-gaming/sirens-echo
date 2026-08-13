# HTTP rejection contract

Every rejection below is a caller error answered with a `4xx`, so a malformed
request is never counted against the service's own error rate.
`internal/community/http_test.go` asserts each row.

## Rejections

| Request | Response |
| --- | --- |
| any method but `POST` | `405` with `Allow: POST` |
| body that is not a JSON object | `400` `request body must be a JSON object` |
| absent, empty, or blank `content`, with no `prompt` | `400` `content is required` |
| `author` over 256 runes, or `content` over 16000 runes | `400` `author or content is too long` |
| `history` longer than `max_context_messages` | `400` `history exceeds the configured context limit` |
| body over 64 KiB | `400`, reported as malformed JSON |
| `prompt` with an empty server or name | `400` `prompt requires a server and a name` |
| `prompt` naming an unrostered server | `400` naming the server the caller supplied |
| unknown path | `404` |
| `POST /healthz` | `405` with `Allow: GET` |

The caps count runes rather than bytes, so a multibyte author is not charged for
bytes it did not spend. The `history` limit is inclusive, so a caller filling the
configured context exactly is admitted.

An unrostered `prompt` server is named back to the caller because the caller
supplied it. A transport failure stays generic instead, so a resolution error
cannot carry an endpoint, host, or port.

## Tolerated behavior

Each of these is pinned by a characterization test that fails when the behavior
changes, so the issue that fixes it has a test to flip rather than delete.

* A body over the 64 KiB cap is refused, but `MaxBytesReader` surfaces through
  the decoder, so the caller is told its well-formed body was malformed.
  [Issue 157](https://forgejo.coilysiren.me/coilyco-gaming/sirens-echo/issues/157)
* Decoding is not strict, so an unknown field is accepted in silence rather than
  refused.
  [Issue 173](https://forgejo.coilysiren.me/coilyco-gaming/sirens-echo/issues/173)
* `X-Sirens-Caller` splits the per-user tier alone, so one HTTP caller can shed
  another's turn through the shared pending counter. Documented rather than
  changed: the header is caller-asserted, so isolating a second tier on it would
  mint budgets rather than bound them.
  [Issue 182](https://forgejo.coilysiren.me/coilyco-gaming/sirens-echo/issues/182)

## Retry-After

Every `429` carries `Retry-After`, including a pending-cap shed. The shed path
charges its own one-second bucket, so the advertised wait is a real window
rather than a constant.

`TestQueueDenialCarriesRetryAfter` asserts both halves: the header is present,
and the value does not exceed the shed window.

## See also

See [the private HTTP entrypoint](sirens-echo-http.md) and
[admission control](sirens-echo-admission.md).
