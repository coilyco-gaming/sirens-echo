#!/usr/bin/env bash
# Builds the Sirens Echo image without publishing, so a Dockerfile or build
# script fault fails a pull request instead of the merge. See docs/deploy.md.
#
# The job container gets no DOCKER_HOST and no mounted socket. The runner's own
# tcp://localhost:2375 is the runner container's loopback, not this one. dockerd
# listens on 0.0.0.0:2375 in the runner pod, so the daemon answers on this
# container's default gateway.
set -euo pipefail

# default_gateway reads /proc/net/route, whose addresses are little-endian hex.
default_gateway() {
  local raw
  raw=$(awk '$2 == "00000000" && $8 == "00000000" { print $3; exit }' /proc/net/route)
  [ -n "$raw" ] || return 1
  printf '%d.%d.%d.%d' \
    "0x${raw:6:2}" "0x${raw:4:2}" "0x${raw:2:2}" "0x${raw:0:2}"
}

# resolve_docker_host returns the first candidate that answers a version call.
# Derived before hardcoded, so renumbering the bridge does not break the build.
resolve_docker_host() {
  local candidates=() gateway
  [ -n "${DOCKER_HOST:-}" ] && candidates+=("$DOCKER_HOST")
  if gateway=$(default_gateway); then
    candidates+=("tcp://${gateway}:2375")
  fi
  candidates+=("tcp://172.17.0.1:2375" "unix:///var/run/docker.sock")
  local candidate
  for candidate in "${candidates[@]}"; do
    if DOCKER_HOST="$candidate" timeout 20 docker version >/dev/null 2>&1; then
      printf '%s' "$candidate"
      return 0
    fi
    echo "docker daemon not reachable at ${candidate}" >&2
  done
  return 1
}

if ! DOCKER_HOST=$(resolve_docker_host); then
  echo "no reachable docker daemon, so the image cannot be built here" >&2
  echo "run scripts/ci-docker-probe.sh in the same job to see what it can see" >&2
  exit 1
fi
export DOCKER_HOST
echo "building against ${DOCKER_HOST}"

# No tag anyone can push and no --push. This proves the build, nothing else.
# The publisher on the deploy runner remains the only thing that ships an image.
docker build --pull=false -t sirens-echo:pr-check .
echo "image build succeeded"
