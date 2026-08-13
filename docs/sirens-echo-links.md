# Outbound link policy

A reply may contain a URL only when that URL appears verbatim in the approved
registry. A link written from memory is a fabrication even when the address
turns out to be real, so an unlisted link is never preferable to no link.

## The registry

The registry is knowledge rather than code. It lives in the
`sirens-echo-knowledge` policy root, split to stay under the documentation size
cap, and both files render into the prompt.

- `references/links-eco.md` carries the official Eco wiki pages and the Eco
  live data surfaces.
- `references/links-community.md` carries the community operator surfaces and
  the one built address, the plain English Wikipedia article form.

The Eco wiki entries were taken from the reviewed index published at
https://eco-app.coilysiren.me/wiki rather than recalled, and every other
address was requested and returned a response before it was written down.

A link belongs after the information it supports, never in place of that
information. One link is the ordinary case and two is the maximum. Linking is
not a substitute for the knowledge-gap path: when approved knowledge cannot
answer, the reply captures the gap instead of gesturing at where an answer
might live.

## Prose masking

A registry is only useful if the runtime accepts what it lists. `maskURLs` in
`internal/community/decision.go` replaces every link span with a plain word
before the style and channel checks run.

Without that step the first-person expression matched the `me` ending every
`coilysiren.me` host, so the runtime rejected every approved link the service
can publish. A URL fragment was read as an invented channel for the same
reason. Both defects were found twice on the same day, once from the link side
and once from the issue-reporting side, because the two share this line.

A registry host therefore needs a test, not an assumption.
`TestValidateNeutralStyleAcceptsRegistryHosts` covers one address per host
family the registry can publish.

## Gate

`agent/evaluation.yaml` scores two cases through `required_patterns`.

- `approved-wiki-link` asks a Housing question and requires either the Housing
  wiki page or the wiki index.
- `approved-live-surface-link` asks where open trades are visible and requires
  the trades page.

Both accept every link the policy would call correct, so neither can fire on a
correct reply. Cases needing an MCP roster are deliberately absent, because a
link case must not fail for a reason that is not the model's.
