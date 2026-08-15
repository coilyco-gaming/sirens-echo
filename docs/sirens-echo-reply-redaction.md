# Redacting a block instead of the message

A reply check refuses. That used to refuse the whole message, so one block
naming a channel nobody has discarded eleven correct blocks beside it. See
sirens-echo#796.

Redaction is the last rung. First [repair](sirens-echo-reply-repair.md) gives
the model a chance to fix the block. Only when that fails does the block go.

## The unit is a block

A block is a paragraph or one item of a list. The blank line is the break a
member already reads, and a list item is a break too, because a bulleted answer
has no blank lines to cut at. The reply this was written for was twelve bullets
with no blank line in it.

A sentence was the obvious unit and is the wrong one. `[.!?]+` splits prose, not
reasoning, so a claim and the qualifier bounding it land in different sentences
and only one gets removed. A block holds both.

## Only rules a block can carry alone

`redactableRules` is an allowlist:

* the four grounding rules - the block invents a channel or claims an action, a tracker write, or work continuing past the turn
* tool-call markup - the markup is in a block
* self-attributed claim - already evaluated per sentence

Everything else refuses whole. `response_style` and `identity_claim` are
properties of the whole voice. `parse` means there was no reply to cut.
`identifier_disclosure` looks redactable and is not: it collapses digits and
spelled numbers across the entire reply, so a guarded value can span two blocks
and neither block fails alone.

## What actually decides

Two passes, and the second is the authority.

The first runs the checks over each block alone and marks the blocks failing
**the same rule that refused the message**. Tying it to that rule keeps a block
that merely disagrees in isolation about something else from being removed.

The second runs every check again over the surviving prose. What is delivered
passed the full set on its own, not merely lacked the removed block. That is
what makes an imperfect first pass safe: it picks candidates and licenses
nothing. If the remainder fails, the reply is refused whole, as before.

It stops when there is nothing to save (one block is the message), nothing left
(every block failed), or too much to remove (`maxRedactedBlocks`).

## The mark

Marked in place, in the harness [notice](sirens-echo-notices.md) shape, whose
alphabet a model reply cannot forge. Adjacent removals share one mark.

```
- forgejo - answered, 116 open issues on sirens-echo.
- playwright - down, same notifications/initialized: Bad Request.
> `content removed by a response check`
```

That shape was always meant for this, though every other notice replaces a
reply rather than standing inside one. The mark is part of the answer, not a
service suffix, so length truncation can reach it when the removed block was
last. Only the disclosure is ever at risk, never the removal.

## What this does not answer

**A remainder can still depend on what was removed.** A summary whose third
bullet was invented is not eleven-twelfths correct, and re-validation checks
rules rather than reasoning. The mark is the mitigation: the member is told the
answer is incomplete rather than left to assume it is whole. The mark also
invites a question the harness cannot answer, judged the smaller cost against
discarding a correct outage report.

## What an operator sees

`response.check` names the rule, so a redaction still appears in the refusal
rates. `response.redacted.blocks` carries the count on every turn, zero
included. `response.check.redacted` logs the rule, the sentence, and the count.
