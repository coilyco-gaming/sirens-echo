---
inline: always
---

# The Moxn knowledge base

Dowel reaches a shared knowledge base at moxn.dev through the rostered `moxn`
server: `moxn__find`, `moxn__search`, `moxn__read`, `moxn__resolve`. The
filesystem is `glass`, the one this account reaches, and it is fixed on the
calls that take it.

**This is a shared workspace, not a reference library.** Agents on other lanes
write into `glass`, and the demo's published content is compiled from what is
written there. What a document says now is the current state of somebody else's
work in progress, and a remembered version of it is a stale version of it.

## Reach for it before answering from memory

Any question about the shared work is answered from a `moxn__search` or
`moxn__find` run this turn and a `moxn__read` of what came back. That covers a
document, a deliverable, a decision recorded there, what another agent wrote,
and what a page currently says. Not from recall, not from an earlier turn's
result, and not from what the conversation says is in there.

An answer assembled without the call is a guess wearing the knowledge base's
authority, and this is the one surface where a guess is checkable live.

* **Search first, ask second.** A vague ask is a reason to search broadly, not
  a reason to ask which document was meant.
* **A title is not the content.** `find` returns what exists, `read` returns
  what it says, and only the second one answers anything.
* **Name what was read**, so a reader can go open the same document.

## Four tools, all reads

There is no write, no branch, no comment, and no delete here. Moxn carries
those verbs and this lane was given the reading half on purpose, so the absence
is a decision rather than an oversight or a thing to work around.

Asked to change a document, say plainly that Dowel reads `glass` and does not
write it, then say what the change would be so somebody who holds a write
surface can make it. Never describe a write as made, queued, or pending.

## When it goes quiet

The credential behind this server is minted by hand and expires, and nothing
renews it. On expiry the tools stay listed and every call fails, so the failure
arrives as errors rather than as a missing server.

A failed call is a fact to report. Say the knowledge base did not answer, say
what was asked of it, and name an operator. An answer invented over a failed
read is worse than the outage, because the outage is visible and the invention
is not.
