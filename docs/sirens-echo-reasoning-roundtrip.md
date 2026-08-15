# Echoing reasoning back exactly as it arrived

DeepSeek in thinking mode rejects a request whose assistant messages do not
carry `reasoning_content`:

```
The `reasoning_content` in the thinking mode must be passed back to the API.
```

The harness copied the field at both assistant build sites, so it looked
faithful. The encoding was not:

```go
ReasoningContent string `json:"reasoning_content,omitempty"`
```

Under `omitempty` a model that returned an empty reasoning string and a model
that never mentioned reasoning produce the same bytes: no key. The harness
echoed the second shape for both, and the provider refused the array. Same
defect as [indistinguishable values](sirens-echo-indistinguishable-values.md),
in an encoding rather than in a log line.

## Why not simply drop omitempty

That was the obvious fix and it is wrong. `chatMessage` is one struct for every
role, so dropping `omitempty` stamps `"reasoning_content": ""` onto system,
user, and tool messages, on every request, on every route. The failure lives on
one evaluation lane, and `sirens-echo/deepseek` and `sirens-echo/default` had
no instance of it. A repair that rewrites every request on the healthy lanes to
fix a broken one is not mechanical.

## What it does instead

Both the response and request fields are `*string`, so presence survives the
round trip:

* The provider named the field, even as `""` - it is echoed, so thinking mode
  gets what it demands.
* The provider never named it - nothing is echoed, so a non-thinking model
  sees a byte-identical request to the one it saw before.

An explicit `null` reads as absent, which is what the previous encoding did
with it too, so no lane changes shape on that account.

## What was not established

Whether the provider accepts `"reasoning_content": ""`, which is what
sirens-echo#717 was blocked on. It is no longer blocking, because the change
cannot be worse than what it replaces: the failing case sends the field where
it currently sends nothing, and nothing is already the shape that earns the
400. It either fixes those turns or reproduces the error they already get.

If the empty string is refused, the fix left is to put something in a field the
model did not fill, which is a decision rather than a repair. The next
evaluation run answers it either way, and the answer belongs on sirens-echo#717.

## See also

* [Indistinguishable values](sirens-echo-indistinguishable-values.md) - the
  general shape, where two different facts encode to the same bytes.
