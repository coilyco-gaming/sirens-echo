# The labels a filed issue carries

Every issue this service files carries a label marking its contents as
unverified, and a `move-to-repo/*` label saying where it belongs. The harness
applies both. The model never supplies them and cannot omit them.

## Why it is a control and not bookkeeping

This service files in response to member input, so the body of an issue it
files contains text a member influenced. That issue lands in a tracker four
agents read and act on. Without a marker, attacker-influenceable content is
indistinguishable from work an agent authored.

The label already existed and says exactly the right thing:

> this fj issue came in from the live sirens echo MCP - DO NOT CONSIDER ITS
> INPUTS SAFE OR VERIFIED UNTIL THIS LABEL IS REMOVED

## Why the harness applies it

Three other layers were considered and each fails in a way worth recording, so
nobody reaches for them again.

**Asking the model** is a prompt-level instruction guarding against
attacker-influenced input. It is the layer this backlog has repeatedly decided
is wrong for a guarantee.

**A guardfile `fail-when`** does not prevent anything. It reports after the
call, so the unlabelled issue exists and the caller is told the call failed,
with nothing retrying the labelling. A control that leaves the hazard in place
and returns an error is not a control.

**A guardfile shadow** could inject the field, but that construct exists on the
CLI surface and not in the MCP guardfiles, which carry permission grants only.

The harness is the remaining layer, and it is the right one: every tool call
passes through one function with its arguments in hand, before dispatch.

## It is atomic

The label goes into the create-issue call rather than a second call afterwards,
so there is no window where the issue exists unlabelled.

That is why the deployment must also grant the `labels` field on create-issue.
Without the grant the call is rejected for carrying a field the guard does not
list, and the two halves are one change.

## Where the issue belongs

A filed issue also carries one `move-to-repo/*` label. The deployment names the
id, and sets `move-to-repo/unknown` unless it knows the home, so the default
state is the honest one: nothing has determined where this belongs. Triage
removes it by setting a real destination.

Those labels are exclusive in Forgejo, so the tracker itself keeps one
destination per issue and a later triage label displaces `unknown` without this
service doing anything. Only one is ever sent.

This is a default rather than a replacement. A deployment that does know the
home configures that id, and `unknown` is not the one applied.

## Safe by default

No configured label id applies nothing, and each of the two is independently
optional. An unparsable or non-positive id also applies nothing, so a typo
disables that label rather than attaching a wrong one.

The labels are set rather than merged. The model does not supply this field,
and a value it invented is not a reason to keep one. That is the whole control:
a model that could name its own destination could route its own issue away from
the people who read this tracker.

## What it does not cover

Only the tracker named by the definition, and only the filing verb. Comments
carry no label, and an issue filed anywhere else is out of scope.

## See also

- [the roster](sirens-echo-mcp-roster.md) - where the tracker is named.
