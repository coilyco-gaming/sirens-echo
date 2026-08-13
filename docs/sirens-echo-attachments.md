# An uploaded file is data, never instructions

A large prompt body arrives as a file and is read through a tool. Splicing it
into the prompt would make every turn carrying a file pay the whole file up
front, which is the problem the feature exists to solve.

## Where it lands

The requester's scratchpad, under `uploads/`. Not a second file concept: path
confinement, the per-file limit, the per-requester quota, and attribution to
the requesting principal all apply because the write goes through the same
reserved path the tool-result spill uses.

`uploads/` is reserved, so the model cannot write there. That matters more than
tidiness. If the model could write into it, it could forge a file and then cite
it as something a member supplied. It is a separate directory from
`tool-output/` for the mirror reason: an upload must not be mistaken for
something the runtime produced.

Reading is already built. `scratch_read` and `scratch_search` are offered
wherever a scratchpad is mounted, and search is the half that scales, since
reading a large file back spends the budget storing it was meant to protect.

## Two different problems in one phrase

"No executable shenanigans, or smuggled rick rolls" names two things, and only
one is solvable.

**Executables are solvable.** The check is the bytes. A null byte or invalid
UTF-8 refuses. The extension and the declared media type both belong to the
uploader and decide nothing.

**Smuggled instructions are not.** A text file is exactly the shape prompt
injection takes, and no filter separates a document discussing instructions
from one issuing them. So the bound is posture rather than detection: the file
is untrusted input always, including from the principal. It never widens
authority, never admits a caller, never names a tool, and a URL inside it is
inert.

## The egress, bounded

Fetching a URL that arrives in a payload is the shape of a server-side request
forgery. Two things bound it. The address comes off the Gateway payload rather
than out of message text, so Discord generated it and a member cannot type one.
And the host allowlist admits only Discord's CDN over TLS, so a payload cannot
choose the destination even if that first property ever stops holding.

An oversized file refuses rather than storing a prefix, because a truncated
document read as a whole one is worse than no document.

## Failing soft

No scratchpad, an unreachable host, a non-text body, an oversized file, or a
partition at quota all leave the turn exactly as it is today: the transcript
still reports that an attachment exists and that its contents were not read.
Losing the upload is much better than losing the answer.

## See also

* [Scratchpad partitions](sirens-echo-scratchpad-partitions.md) - the store.
* [Tool results](sirens-echo-tool-results.md) - the same reserved write path.
