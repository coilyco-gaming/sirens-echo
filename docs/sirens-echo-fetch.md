# The fetch tool

A read-only HTTPS GET the model can call. Plain `net/http`, no dependency. The
fetching is the easy part; the allowlist is the feature.

## Why it needs bounds at all

The fetch runs inside the cluster. Without bounds it can reach the tailnet,
other services' internal endpoints, and cloud metadata addresses — and the
model is precisely the component an attacker gets to talk to. A tool that
fetches any URL a model can be persuaded to fetch is server-side request
forgery with a conversational interface.

## The switch is the deployment

`SIRENS_ECHO_FETCH_HOSTS` is a comma-separated allowlist. **Empty offers no
tool at all**: no schema in the prompt, no mention to the model, nothing to be
talked into. Enabling it is a decision someone makes, the same as the
scratchpad and the content gate.

## Five bounds, and what each one is for

**Exact host match.** Not a suffix. `eco-app.coilysiren.me.evil.example` is a
different host that a suffix check would accept, and registering that domain
costs an attacker nothing.

**HTTPS only.** Plaintext inside a cluster is a different risk with no upside
here.

**Private addresses refused at dial time**, not by reading the URL. This is the
one that is easy to get wrong: an allowlisted hostname can resolve to an
internal address, deliberately or by accident, and a check that only reads the
hostname never sees it. Loopback, private ranges, link-local, and unspecified
are all refused when the connection is made.

**Redirects refused.** A redirect is a second destination the allowlist never
saw, and following one turns an approved host into an open relay.

**A size cap and a timeout.** A large body becomes prompt, and a slow host
otherwise spends the whole turn.

## GET only

A curl-shaped tool that reads is most of what was asked for. One that writes is
a different authority and should be requested on its own terms rather than
arriving as a side effect of this.

## The boundary this does not solve

A fetched page is untrusted text entering the prompt. The allowlist bounds
*where* it comes from and says nothing about *what it says*, so an approved
host serving hostile instructions is still an open question. That is the same
boundary web search raises and it belongs with it, not here.

## See also

- [tuning numbers](sirens-echo-tuning.md) - the size cap and the timeout.
- [the content gate](sirens-echo-content-gate.md) - classifies a member's
  request, and deliberately not tool output.
