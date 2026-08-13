# Answering questions about itself

Members ask what the service is and how it works. That answer is the one the
service is worst at, because it sounds like knowledge and is actually memory of
a prompt. Three rules narrow it to what can be checked.

## Link the source, do not recite it

The repository is public, so a path is a link and a link is verifiable by the
person reading it. That is the whole value: the member can go and look.

The link may only be offered for a path already named in the conversation or
returned by a tool. A model asked where something lives will otherwise produce
a plausible path, and a plausible path is a fabrication with a URL on it.

Quoting is stricter still. Quote source only from text a tool returned. Naming
a file is not reading it, and no roster today offers a tool that reads file
contents, so a described file is a guess about a name.

## A link is not the running build

The obvious link points at `main`. The running process is a pinned image, and
the two differ whenever `main` has moved since the last successful publish,
which is routinely.

The process also cannot close that gap itself. The image is built without a
revision: the Dockerfile copies `cmd`, `internal`, `agent`, the skill roots,
and `docs`, and no `.git`, so the toolchain stamps no `vcs.revision`. There are
no `-ldflags`. Nothing reads `runtime/debug`.

So the honest form is that the link is current source, not the code that
answered. Saying otherwise invents a provenance the process does not have.

## Its own runtime is invisible

No logs, traces, metrics, uptime, restarts, or error rates. A question about
how the service is behaving is a question for an operator.

This is worth stating because it is the tempting answer. Asked whether it is
slow today, a model will produce a number shaped like an observation.

## The guards

`TestCapabilityDocLinksThisRepository` ties the link form to the module path in
`go.mod`, so moving the repository breaks the doc loudly rather than leaving
members a dead link.

`TestCapabilityDocIsRightThatTheBuildCarriesNoRevision` reads the Dockerfile
and fails if the build gains a `.git` copy or a linker assignment. Adding
revision stamping is a good change. It just has to update the sentence that
currently tells the model the revision is unknowable.

Both were mutation-checked rather than trusted. The second one silently
skipped on its first version, because the phrase it matched wraps across a line
in the doc, so it now matches against reflowed text.

## What is deliberately not here

The SigNoz half of the original request. Deep's roster grants forgejo, steam,
and demo-discord, so there is no telemetry surface to answer from. Encouraging
the model toward a tool it is not offered is the over-claiming defect this file
exists to prevent, so the doctrine states the absence instead.
