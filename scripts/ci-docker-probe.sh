#!/usr/bin/env bash
# Reports what a job container can see of the runner's dind daemon, for a failed
# image-build. See docs/sirens-echo-deploy.md.
set -u

script_dir=$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)
# shellcheck source=scripts/lib/docker-host.sh
. "${script_dir}/lib/docker-host.sh"

# Leads with the build's own resolution, so a Dockerfile fault does not read as
# a daemon outage. See docs/sirens-echo-deploy.md.
echo "== what the build resolved =="
if [ -r "$DOCKER_HOST_RECORD" ]; then
  echo "the build reached a daemon at $(cat "$DOCKER_HOST_RECORD")"
  echo "so resolution succeeded and the fault is later in the build"
else
  echo "no daemon recorded, so the build failed at or before resolution"
fi

echo "== docker-related environment =="
env | grep -i docker || echo "no docker variables"

echo "== client on PATH =="
command -v docker || echo "no docker client"
docker --version 2>&1 || true

echo "== mounted socket =="
ls -l /var/run/docker.sock 2>&1 || echo "no socket"

echo "== default gateway =="
if gateway=$(default_gateway); then
  echo "$gateway"
else
  echo "no default route"
fi

echo "== the addresses the build tries, in order =="
while read -r host; do
  echo "-- trying $host"
  DOCKER_HOST="$host" timeout 15 docker version --format '{{.Server.Version}}' 2>&1 ||
    echo "unreachable"
done < <(docker_host_candidates)

echo "== probe complete =="
exit 0
