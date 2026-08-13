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
prose. A partial roster is a live condition, and nothing in the reply path strips
this markup.

## What it catches, and what it misses

The target is the delimiter syntax, not the words. Prose about tool calls and a
quoted JSON field are correct and common, so these stay clean:

```
The harness emits tool_calls as a structured field rather than as content.
I cannot call list_issue, since that tool is not in my roster.
Here is what a tool call looks like in the proxy log: "tool_calls": [...]
```

**The name set is closed around the wrong thing, and this is measured.** A probe
of 5 live turns asking Deep to file an issue emitted markup in 4 of 5 replies and
this check caught **none** of them:

| Emitted | Caught |
| --- | --- |
| `<｜｜DSML｜｜tool_calls>` | yes |
| `<create_issue> { "title": ... }` | **no** |
| `<create_issue> <title>...</title>` | **no** |
| `<tool_round> { "name": ... }` | **no** |

The pattern matches a closed set of names taken from published formats:
`tool_call`, `tool_calls`, `function_calls`, `invoke`. The model does not use
those. It builds the tag from the tool's own name, or from its own notion of a
round. So this covers one observed family and misses at least two others from the
same model on the same route.

**Treat a green result as no evidence.** Echo's `ornith:35b` form is also
unverified, because the tower was wedged
([deploy#437](https://forgejo.coilysiren.me/coilyco-bridge/deploy/issues/437)).

## What would work

A tag whose name is a tool name, which is a value from configuration rather than
a word from a vocabulary. That is why `checkPrincipalEcho` survives translation
while English word lists do not, recorded on
[sirens-echo#253](https://forgejo.coilysiren.me/coilyco-gaming/sirens-echo/issues/253).
It needs a must-not-fire corpus from live replies before a pattern.

## Why it is opt-in

The rate depends on the request. A case that does not ask for an action produced
1 of 5; asking for one produced 4 of 5. Action-shaped requests are the trigger.

An always-on check would turn the deployment gate flaky on a non-security
behaviour, which [the battery](sirens-echo-battery.md) exists to prevent, and the
recorded gating policy is that security cases gate and everything else reports.
It runs last in `runScopedChecks`, leaving every existing precedence unchanged.

## Accepted false positive

A reply that quotes these delimiters while explaining them is a finding, bounded
by the check being opt-in.
