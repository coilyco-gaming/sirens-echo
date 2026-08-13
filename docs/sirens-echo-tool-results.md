# What happens to a large tool result

One tool result is capped at 8 KiB before it re-enters the prompt, so a round of
parallel calls cannot inflate the context past the model's budget. The cap is
not the interesting part. What happens to the rest is.

## The remainder used to be destroyed

The full result stays on the executed-tool record for telemetry and grounding,
but only the capped copy reaches the model. Everything past the cut was
unreachable for the rest of the turn, so a large payload was silently beheaded
and the model answered from its first 8 KiB with no way to know more existed.

## The remainder is now saved

When a result is trimmed and the deployment mounts a scratchpad, the runtime
writes the whole result to the requester's own scratchpad and appends a line
naming the file and the true byte count. The model can then read the rest with
`scratch_read`, or search it with `scratch_search`, instead of guessing.

The save goes through the `scratch_write` tool rather than the filesystem, so
path confinement, the per-file limit, the per-requester quota, and attribution
to the requesting principal all apply exactly as they do for a model-requested
write. Nothing about the scratchpad's boundary is special-cased for this.

Saves are numbered per turn, so a second call to the same tool cannot overwrite
what the first one saved.

## Every failure falls back rather than failing the turn

Three cases produce no save, and all three leave the previous behavior exactly
as it was, a trimmed result carrying the truncation marker:

- the deployment mounts no scratchpad, so the write tool is not offered
- the result is larger than the scratchpad's per-file limit
- the requester's partition is already at its quota

Losing the remainder is much better than losing the answer, so none of these
reach the member as an error.

## What the file name can be

The file name is built from the tool's own name with everything outside letters,
digits, hyphen, and underscore removed, under a single `tool-output` directory.
A tool name is server-supplied, so it is treated as untrusted input and can
never reach the filesystem as a path. The scratchpad's own confinement refuses a
traversal independently, which means the flattening is the first of two gates
rather than the only one.

## See also

* [Scratchpad](sirens-echo-scratchpad.md) - the filesystem the remainder lands in.
* [Tools](sirens-echo-tools.md) - how a tool surface reaches the model.
* [Observability](sirens-echo-observability.md) - `mcp.tool.result.bounded` carries
  the saved path.
