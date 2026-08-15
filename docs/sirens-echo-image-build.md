# Building the image on a pull request

The `image-build` job runs `scripts/ci-image-build.sh`, which builds the root
Dockerfile with a tag nobody can push and no `--push`. A build-context fault
therefore fails the pull request rather than the merge that carries it.

## Finding the daemon

That job container gets no `DOCKER_HOST` and no mounted socket. dockerd listens
on `0.0.0.0:2375` in the runner pod, so the daemon answers on the container's
default gateway, and the script has to work out which address that is.

`scripts/lib/docker-host.sh` derives the candidates: an inherited `DOCKER_HOST`
first, then the default gateway decoded from `/proc/net/route`, then the
conventional `172.17.0.1`, then the socket path. Deriving the gateway ahead of
the hardcoded address is what lets the bridge renumber, which it has. One run
built against `172.18.0.1`.

`/proc/net/route` stores addresses as little-endian hex, so `010012AC` is
`172.18.0.1`. The decode lives in `default_gateway` rather than in a reader's
head.

## Reporting a failure

`scripts/ci-docker-probe.sh` runs only when `image-build` fails, and reports
what the job container can see. It reads the same candidate list, so it can
never report on an address the build would not have tried, and can never miss
the one the build chose.

The build records the address it settled on, and the probe leads with that
record. Most `image-build` failures are Dockerfile faults rather than daemon
faults, and a bare list of candidates with some marked unreachable reads like a
partial outage that did not happen. Naming the daemon the build actually reached
says plainly that resolution was not the fault.

The probe never fails, because its caller wants the report rather than a second
red step. That is why it is the one script here without `-euo pipefail`.

## See also

* [Deployment](deploy.md) - what publishes an image, and where it runs.
* [Reviewed skip set](sirens-echo-test-skips.md) - the other place a step that
  cannot fail is accounted for.
