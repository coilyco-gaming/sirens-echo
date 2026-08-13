# Encoded capability limits

`.agents/skills/sirens-echo-knowledge/references/capability.md` tells the model
what the service can do. This file is the reviewer's copy and cites the code
that sets each bound, so a change that moves a number surfaces as a stale
document rather than a stale belief.

The model-facing file carries no citations on purpose. They cost prompt bytes
and mean nothing to a model, and a reader who needs them is reading this file.

## The bounds and their sources

| Bound | Value | Source |
| --- | --- | --- |
| Tool rounds per turn | 6, then the turn fails | `internal/community/proxy.go:21` and `:411` |
| Tool execution | Sequential, fail-fast | `internal/community/proxy.go:428` |
| Reply size | 1800 characters | `internal/community/decision.go:47` |
| History supplied | 12 messages | `agent/sirens-echo.yaml` |
| Job kinds | `echo` and `ward-exec` only | `internal/community/jobsubmit.go:24` |

Nothing schedules a turn. Every turn originates from a Discord message or an
HTTP request, so no reply may describe work continuing after it is sent.

## Why the grammar is named explicitly

The reports behind this were not the model claiming a tool it lacked. They were
the model describing an aspiration in the grammar of a shipped capability, as
in "the system is now processing these requests sequentially". A prohibition on
inventing tools does not reach that sentence, because it names no tool.

So the policy forbids the continuing-action forms directly and the case scores
them, rather than trusting a general honesty instruction to cover a specific
construction that has already shipped once.

## What this deliberately omits

The asynchronous job surface is real in the binary and is left out of the
model-facing file. Nothing in the deployed Echo values enables a job store for
this lane, so listing it would advertise a capability the community profile may
not have. Adding it needs a deployment fact rather than a code reading.

Sirens Deep is not covered. It loads `coilyco-general` and never sees this
policy root, so the same defect can recur there against a different file.
