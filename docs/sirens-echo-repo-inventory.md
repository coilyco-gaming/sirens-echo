# Public repository inventory

`list_public_repos` lists an organization's public repositories with their
description, URL, language, and when each was last updated. It answers "what
does this org have" without anyone pasting a list into a prompt, which is the
same staleness problem a written inventory always has.

## The switch is the deployment

`SIRENS_ECHO_REPO_INVENTORY_URL` and `SIRENS_ECHO_REPO_INVENTORY_ORG`. Either
empty offers **no tool at all**: no schema in the prompt, no mention to the
model, nothing to be talked into. The same posture the fetch tool and the
scratchpad take.

## Public because it holds no credential

The read is unauthenticated. No `Authorization` header is set anywhere in
`repoinventory.go`, and a test asserts that none is sent.

**That is the guarantee, rather than a visibility filter.** A filter is a line
of code that can be written wrong, reviewed wrong, or regressed by a later
change. An unauthenticated request cannot see a private repository at all, so
there is nothing for a mistake to leak.

A private or internal record is still dropped if one arrives, because a wrong
token configured somewhere else must not turn into a disclosure here. That
check is the belt; having no credential is the braces.

## It does not need a checkout

The inventory is metadata, so it comes from the forge API rather than from a
mounted tree. It reports what the organization has, not what a mount happens to
contain, and those drift apart the moment a repository is added.

That also means it does not wait on the mount work in
[sirens-echo#633](https://forgejo.coilysiren.me/coilyco-gaming/sirens-echo/issues/633).
Reading source is a separate capability with a separate shape.

## Bounds

The listing is capped at `maxRepoInventoryEntries`, so one large organization
cannot fill a tool result on its own. Entries sort by name, so two calls a week
apart differ only where the org differs.

The dialer is the fetch tool's, which refuses loopback, private ranges,
link-local, and carrier-grade NAT. An inventory pointed at an internal address
fails at connect rather than reaching a cluster service.

## Reading one file

`read_public_file` takes owner, repo, path, and an optional ref. Same forge,
same absence of a credential. A path that climbs out of the repository is
refused before any request is made, and each segment is escaped separately so a
segment carrying a slash cannot forge one.

Output is capped at `maxRepoFileBytes` and says so when it cuts, because a half
file the model cannot tell from a whole one is answered from with ordinary
confidence.

**Neither tool writes anything**, and every URL either returns is one a member
could have opened themselves.

## Why not a mount

[sirens-echo#633](https://forgejo.coilysiren.me/coilyco-gaming/sirens-echo/issues/633)
asked for public repositories mounted ward-style so the agent could read its own
source. These two tools answer that need without a volume: a mounted clone is
stale the moment anything merges, and an API read is current by construction.

A mount still buys what an API read cannot — grep across a whole tree, following
imports, a repository too large to read a file at a time. That question stays
open on 633.

See [the fetch tool](sirens-echo-fetch.md) for the dialer, and
[configuration](sirens-echo-config.md).
