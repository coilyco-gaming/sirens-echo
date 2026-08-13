# Tool-call markup

`forbid_tool_call_markup` rejects a reply carrying the model's own tool-call
delimiters as content. A member reads that verbatim, and no other check in the
family can see it.

## Why it exists

The first live Deep gate run scored this reply as a pass:

```
I'll check the issue tracker in the repo for recent announcements.

<｜｜DSML｜｜tool_calls>
<｜｜DSML｜｜invoke name="list_issue">
<｜｜DSML｜｜parameter name="state" string="true">open</｜｜DSML｜｜parameter>
</｜｜DSML｜｜invoke>
</｜｜DSML｜｜tool_calls>
```

It passed because the case's only forbidden pattern was `https?://`. The reply is
in `evaluations/eval-deep-run1.yaml` and the finding is
[sirens-echo#301](https://forgejo.coilysiren.me/coilyco-gaming/sirens-echo/issues/301).

The model wanted a tool the roster did not carry, so it emitted the call as
prose. A partial roster is a live condition rather than a hypothetical, and
nothing in the reply path strips this markup.

## What it matches, and what it deliberately does not

The target set is the **delimiter syntax**, not the words. Prose about tool
calls and a quoted JSON field are both correct and common, including in this
repository's own debugging threads. These stay clean:

```
The harness emits tool_calls as a structured field rather than as content.
I cannot call list_issue, since that tool is not in my roster.
Here is what a tool call looks like in the proxy log: "tool_calls": [...]
```

Keeping the target closed is what separates this from an unbounded assertion
detector. A reply containing the literal invoke delimiters is the model failing
to emit a structured call, which is never correct.

| Family | Form | Verified |
| --- | --- | --- |
| DeepSeek | `<｜｜DSML｜｜tool_calls>`, U+FF5C rather than an ASCII bar | **observed live** on `deepseek-v4-flash` |
| Hermes, Qwen | `<tool_call>` and `</tool_call>` | from the published format, not observed here |
| Anthropic | `<function_calls>`, `<invoke name=...>` | from the published format, not observed here |
| Llama, harmony | `<\|python_tag\|>`, `<\|channel\|>` | from the published format, not observed here |

**Only the DeepSeek row is measured.** The others are written from published
formats and have never fired against a real reply in this repository. Echo's
route resolves to `ornith:35b`, whose markup form is **unverified**, because the
tower was wedged when this check was written
([deploy#437](https://forgejo.coilysiren.me/coilyco-bridge/deploy/issues/437)).
So a green Echo case is not yet evidence that Echo does not do this. Re-check the
Echo route once that clears.

## Why it is opt-in

The behavior reproduced 1 of 5 live runs. An always-on check would turn the
deployment gate flaky, which is the failure mode
[the battery](sirens-echo-battery.md) exists to prevent, and Kai's recorded
gating policy is that security cases gate and everything else reports.

So the check is a mechanism and the decision to gate on it belongs to whoever
writes the case. A rate-pack case measures how often the model does this. A gate
case would assert it cannot happen, which no current evidence supports.

It runs last in `runScopedChecks`, so adding it left every existing precedence
unchanged. The rate runner attributes a rate to whichever check fired first, so
that ordering matters as much as the check does.

## Accepted false positive

A reply that quotes these delimiters while explaining them, rather than emitting
them, is a finding. That is a real cost and it is bounded by the check being
opt-in: it only reaches cases whose author asked for it. A case exercising
Echo's ability to explain its own tool syntax should not set the flag.
