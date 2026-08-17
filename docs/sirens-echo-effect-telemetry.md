# Recording what the harness itself did

A tool call the model made was fully recorded. **A command this service executed
was not**, and neither was fetching a member's upload. For an audit record that
is the wrong way round: a read-only `eco__get_market` left a trace and running a
process left nothing.

Neither path had a span, a metric, or a log line. They were absent from the
record by being absent from everything.

## Workspace command execution

`job.command` span, and `sirens_echo.commands` counting `command.verb` against
an outcome.

The **verb** is the label - `git` or `exec`, this repository's own closed set.
**Arguments are not**, because a clone argument is a repository URL and a verb
argument is whatever the job kind declared.

Three outcomes, kept apart because an operator reading them wants to know which:

- `ok` - ran and exited zero.
- `exited` - ran and exited non-zero. The code goes on the span, not the metric:
  0 to 255 is cardinality a closed label does not want.
- `did_not_run` - never started at all.

The span carries the duration, the truncation flag, and the job id. The output
never reaches any of them, because it is command output and this service already
refuses to let it reach a member.

## Attachment ingest

`attachment.fetch` span, and `sirens_echo.attachments` counting the outcome.

Every arm records, including the three that used to `continue` silently:

- `refused_host` - not a Discord CDN address, refused before any fetch.
- `fetch_failed` - the CDN did not answer, or answered non-200.
- `refused_binary` - a null byte or invalid UTF-8, so it is not text.
- `write_failed` - the scratchpad refused it.
- `stored` - written and handed to the turn.

**The filename never appears**, which is why the span carries the byte count and
the status code rather than the URL: a CDN path has the member's filename in it.
The content never appears either.

Before this, an upload that was refused and an upload that never happened were
the same observation. `discord.attachment.stored` counted the successes at the
caller and nothing counted the rest.

## It stops at telemetry

**None of this reaches the Temporal mirror**, and that is deliberate rather than
incidental. The mirror keys off `RecordToolCall` alone, so a new `Record*` does
not widen what leaves this process.

Whether a command execution should be exported to a third-party SaaS is a
disclosure decision, and sirens-echo#887's whole design says a person makes it on
purpose. A command verb is closer to content than `mcp.tool.name` is.
`TestNeitherEffectReachesTheTemporalMirror` holds the line, and it asserts a tool
call *does* mirror in the same run so it cannot pass by delivering nothing.

That question is the open half of sirens-echo#890.

## See also

- [the tool mirror](sirens-echo-tool-mirror.md) - what does leave, and why.
- [execution](sirens-echo-execution.md) - the workspace and its bounds.
- [attachments](sirens-echo-attachments.md) - the ingest path itself.
