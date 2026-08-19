---
doc_goal: State what roster re-export offers over /mcp, why it is off by default, and what a caller gets past when it is on.
---
# Roster re-export

`SIRENS_ECHO_MCP_REEXPORT` offers the lane's own rostered tools over `/mcp` beside `turn`, so a fleet
client makes one tool call instead of paying a whole agent turn for it. Off by default. Argued at
sirens-echo#1025.

## What it changes, stated plainly

**A re-exported call is not a turn.** The lane's guards live in the turn pipeline rather than in the
tools: `runReplyChecks`, response validation, and the `IdentifierGuard` that #310 depends on. A caller
reaching `moxn__find` reaches the server, and none of those run.

So this **moves a security boundary rather than adding an interface**, which is the reason for every
choice below.

## The gate

Every re-exported call needs the deployment token in an `Authorization: Bearer` header, compared in
constant time. `turn` is unchanged and still needs none.

**An empty `SIRENS_ECHO_HTTP_TOKEN` trusts nobody**, so turning re-export on without configuring a
token offers tools that refuse every caller. That is deliberate: a half-configured deployment should
fail closed, especially on a NodePort that by design refuses nobody at the network layer.

This is the existing token actually enforced on a new path rather than the real authentication #1025
says probably wants landing first. If that is judged insufficient, the flag staying off is the safe
state.

## Naming

Tools keep the `server__tool` name `proxyToolName` already gives them, so the collision rule guarding
the model's own tool list guards this surface too. Twelve servers offering `find` stay twelve
addressable tools rather than one nobody chose.

## Staleness, and why a failed refresh keeps the old list

The roster is discovered per turn, and #943 recorded it collapsing from 86 tools to 0 and back. A list
cached forever would advertise tools that stopped existing, so the advertised set refreshes on
`SIRENS_ECHO_REEXPORT_REFRESH`, one minute by default.

**A failed refresh keeps the previous list rather than emptying it.** A client whose tool list vanishes
underneath it is the worse failure, and worse here than internally: a lane's view of a server can fail
while other clients use that same server successfully, which reads as a broken tool rather than a
broken path to one.

## Cost

A call opens a roster session, calls, and closes it. That is honest rather than efficient, and it is
the first thing to improve if this carries real traffic. Admission runs through the same `transportMCP`
budget as a turn, so a tool client cannot outspend the guilds it shares a deployment with.

## Deliberately not here

* **No scoping to part of the roster.** Opting in offers all of it. A deployment wanting less trims the
  roster rather than the export.
* **No per-tool authority.** One token answers for the whole surface, so write tools and read tools are
  gated identically.
* Deploy's README still says every MCP is ClusterIP-only with no lane reaching across a namespace. That
  stays true of the servers. This changes what one lane's own listener offers, and that README wants a
  matching edit if any lane turns this on.

## Related

* [HTTP and MCP surfaces](sirens-echo-http.md) - the listener this hangs off.
* [Tools](sirens-echo-tools.md) - the roster being re-exported.
