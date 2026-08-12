# Sirens Echo harness notices

A notice is a member-facing string this service wrote itself. It is never model
output, and the rendered shape is what tells the two apart.

## The shape

Every notice renders as one blockquoted code span:

> `rate limit exceeded`

> `http 404 not found`

> `eco tool not available`

The blockquote and the code span are literal. They are visual styling, not a
description of styling, and both are required.

## The phrase

The phrase inside the span is the short technical form a semi-technical member
recognizes. It states the condition the way a status line does.

A notice is not a sentence. `http 500 internal server error` is a notice. "the
http server had an internal server error" is prose, which is what a model reply
sounds like, which is the confusion the format exists to prevent.

The alphabet is lowercase letters, digits, spaces, and `, . / -`. Anything else
is stripped before rendering, so no phrase can close the code span early,
inject markdown, or span two lines.

## Where they live

`internal/community/notice.go` holds every phrase the service can emit. A new
condition adds a phrase there rather than a string at the call site, and a test
asserts each rendered notice against the shape.

`harnessNotice` is the only constructor. It lowercases, collapses whitespace,
strips the disallowed alphabet, and falls back to a fixed phrase when nothing
usable survives, so a caller bug still reaches the member as a notice.

## Failure classes

A failed turn always replies. The class is chosen from the stage that failed
and the cause, because the member's next useful move differs per class.

| condition | notice |
| --- | --- |
| turn deadline expired | `turn timed out, retry shortly` |
| MCP surface or tool call failed | `tool call failed` |
| channel history unreadable | `channel history unavailable` |
| model or Agent Proxy failed | `model backend unavailable, retry shortly` |
| reply failed grounding or style | `reply blocked by response check, rephrase` |
| anything else | `turn failed` |

A deadline and a tool failure outrank the stage, since both name the surface to
stop waiting on more precisely than the stage does.

The notice is sent on a context detached from the turn deadline. A turn that
failed by expiring has no budget left to say so otherwise, which is how the
slowest failures used to end as silence.

No model round trip is involved. The error path cannot inherit a model failure
it was written to report.

## What a notice never carries

A notice carries a condition class and nothing else. Model output, prompt text,
tool payloads, MCP endpoints, stack detail, member identifiers, and internal
error strings all stay out of it.

That boundary is why the phrases are a closed set. A formatted upstream error
would put an arbitrary internal string in front of a member.

Response style validation binds model replies and not notices, since a notice
never reaches the model or passes through `ParseReply`. The shape here is the
equivalent guarantee for the strings the harness writes.

See [the prompt](sirens-echo-prompt.md), [admission
control](sirens-echo-admission.md), and [the service](sirens-echo.md).
