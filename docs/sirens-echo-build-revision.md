# Which commit is answering

The running binary can name the commit it was built from. Without that, a
question about the running build is answered from deployment state rather than
from the process, and every evaluation dataset records an unnamed image.

## How it gets there

The publish script already derives the full source sha to tag the image. It now
passes that same value as a build argument, and the build stamps it into a
package variable with a linker assignment. One source of truth, used twice.

`.git` is deliberately not copied into the build stage. Letting the toolchain
stamp `vcs.revision` on its own would work, but it adds the whole repository to
the build context to recover one string, and it would make the revision depend
on how the tree was checked out rather than on what the publisher tagged.

## An unstamped build says so

A local `docker build` or a `ward exec image` run passes no argument, so the
revision is empty and `BuildRevision` returns an empty string. That is the
honest answer for a build that carried no revision, and it is why readiness
records the value rather than a placeholder: an empty field is visibly
different from a wrong one.

## What it does not license

Naming a commit does not let the process read that commit. A source link is
still current source rather than the code that answered, unless the revision is
named alongside it. The capability ledger says so, and the claim is bound to the
Dockerfile by a test: a doc asserting the build carries no revision fails once
the build gains a linker assignment.

## See also

* [Capability ledger](../.agents/skills/sirens-echo-knowledge/references/capability.md)
* [Rollout](sirens-echo-rollout.md) - how the tag and the image relate.
