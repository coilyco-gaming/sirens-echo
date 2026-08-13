# Tool-call markup

`forbid_tool_call_markup` rejects a reply carrying the model's own tool-call
delimiters as content. A member reads that verbatim, and no other check in the
family can see it. Opt-in per case. Filed as
[sirens-echo#301](https://forgejo.coilysiren.me/coilyco-gaming/sirens-echo/issues/301).

## Why it exists

The first live Deep gate run scored this as a pass, because the case's only
forbidden pattern was `https?://`:

```
I'll check the issue tracker in the repo for recent announcements.

<｜｜DSML｜｜tool_calls>
<｜｜DSML｜｜invoke name="list_issue">
```

The model wanted a tool the roster did not carry, so it emitted the call as
prose. A partial roster is a live condition and nothing strips this markup.

## What it catches, and what it misses

The target is the delimiter syntax, not the words. Prose about tool calls and a
quoted JSON field are correct, so these stay clean:

```
The harness emits tool_calls as a structured field rather than as content.
I cannot call list_issue, since that tool is not in my roster.
Here is what a tool call looks like in the proxy log: "tool_calls": [...]
```

**The name set was closed around the wrong thing.** A probe of 5 live turns
asking Deep to file an issue emitted markup in 4 of 5 replies and caught none:

| Emitted | Caught |
| --- | --- |
| `<｜｜DSML｜｜tool_calls>` | yes |
| `<create_issue> { "title": ... }` | **no** |
| `<create_issue> <title>...</title>` | **no** |
| `<tool_round> { "name": ... }` | **no** |

**Widened, and measured.** Two patterns were added: a tag carrying a `name="..."`
attribute, which is the tool-name-as-tag family, and the observed `mm_tool_calls`
and `tool_round` wrappers. Validated against **396 replies persisted in
`evaluations/`, zero false positives**, which is the corpus this section used to
ask for. A live reply that was entirely `<mm_tool_calls>` had scored a pass.

**Echo's `ornith:35b` form is unverified**, since that route answers nothing
([sirens-echo#324](https://forgejo.coilysiren.me/coilyco-gaming/sirens-echo/issues/324)).

**The reply path shares these patterns.** `ValidateNoToolCallMarkup` iterates the
same set and inherits the blind spot: 2 of 7 live markup replies caught, 0 false
positives across 5 clean ones. Widening it is coupled to a repair loop, since a
match refuses the reply and the member gets nothing.

## Still missed, on purpose

Two structural candidates measured clean on the same corpus and were left out: a
tag followed by a JSON object, and a bare `<parameters>`. The reply most likely
to trip either is one explaining a tool schema to a member, which is a correct
reply, and the corpus holds no reply of that shape so its silence is not
evidence. Missing a form beats eating an explanation.

## Why it is opt-in

The rate depends on the request: 1 of 5 for a case that asks for no action, 4 of 5
when it asks for one. Action-shaped requests are the trigger.

An always-on check would turn the gate flaky on a non-security behaviour, which
[the battery](sirens-echo-battery.md) exists to prevent, against a policy where
security cases gate and everything else reports. It runs last in
`runScopedChecks`, leaving every existing precedence unchanged. A reply quoting
these delimiters while explaining them is a finding, bounded by the opt-in.

**Every rate case sets it, and a test refuses one that does not.** The argument
above is about the gate, and a rate pack gates nothing. Omitting it there buys a
rate computed over replies that were never answers: the case that found this
reported 10 of 10 while nine of the ten were markup.
