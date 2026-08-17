# Reading the model call as a stream

A turn that was queued and a turn that was hung produced the same thing from
here: silence, then a deadline. The turn died at the same second either way, and
the member was told it timed out in both.

## Two bounds instead of one

A **total** deadline fires on schedule however many bytes arrived, so it cannot
tell those apart by construction. The call now carries two bounds:

* **The turn context** is the ceiling. It is the whole turn's budget and it did
  not move here. Choosing that number is sirens-echo#577's.
* **`SIRENS_ECHO_MODEL_IDLE_TIMEOUT`** bounds *silence*, not the call. Any line
  resets it, and a heartbeat is a line.

`http.Client.Timeout` is deliberately unset. It is a total timeout, so leaving it
would have cut a streaming completion on schedule and reintroduced the defect one
layer down.

## What activity looks like

`stream: true` is required, because heartbeats have nowhere to go on a
non-streaming request. Agent Proxy emits them as SSE comments:

```
: {"state":"attempt","n":1,"of":2,"backend":"tower-3026","regime":"idle"}
: {"state":"upstream_started","backend":"tower-3026"}
data: {"choices":[{"delta":{"content":"Paris"}}]}
data: {"choices":[{"delta":{},"finish_reason":"stop"}]}
data: [DONE]
```

A keepalive repeats the current state, so a state that persists keeps the
connection provably alive without a transition. Only `attempt` is logged, as
`model.attempt`: a ten-second beat under a five-minute ceiling would otherwise be
thirty log lines saying nothing changed. The count reaches `model.response` as
`heartbeats`.

An unparsable comment is skipped rather than fatal. The `data:` frames are the
contract and a malformed hint must not cost the turn.

## Assembling the answer

Content and reasoning concatenate. Tool calls arrive as fragments keyed by
`index`, so name and argument pieces are joined per call, and the calls are
returned **in the order their indexes first appeared** rather than sorted, which
is the order the model asked for them in.

Reasoning stays unnamed when the stream carried none. An absent field and an
empty one are echoed back differently on the next round.

## The non-streamed shape still works

A backend that answers a streaming request with a whole JSON body is read as one,
selected on `Content-Type`. Agent Proxy's own non-streaming surface still exists,
and failing a turn that arrived intact would be a worse trade than keeping both
paths. No heartbeats reach that path, so the turn ceiling is its only bound,
exactly as before.

## The two failures are different sentences

| what happened | error | notice |
| --- | --- | --- |
| nothing arrived at all | `ErrModelSilent` | `model backend went quiet, retry shortly` |
| still working at the ceiling | `context.DeadlineExceeded` | `turn timed out, retry shortly` |

The split is the point. Retrying silence can work. Retrying a turn that ran the
ceiling out while making progress reproduces it, which is the misdirection
sirens-echo#449 names. `failureCause` reports `model_silent` against `timeout`,
and the span carries `sirens_echo.model.silent`.

## See also

- [tuning a deployment](sirens-echo-tuning-overrides.md) - the idle timeout knob.
- [model retry](sirens-echo-model-retry.md) - what is retried, and why silence is.
