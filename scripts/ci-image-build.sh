#!/usr/bin/env bash
# Builds the Sirens Echo image without publishing, so a Dockerfile or build
# script fault fails a pull request instead of the merge. The job container gets
# no DOCKER_HOST and no mounted socket, so the daemon address is derived rather
# than given. See docs/sirens-echo-deploy.md.
set -euo pipefail

script_dir=$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)
# shellcheck source=scripts/lib/docker-host.sh
. "${script_dir}/lib/docker-host.sh"

# resolve_docker_host returns the first candidate that answers a version call.
resolve_docker_host() {
  local candidate
  while read -r candidate; do
    if DOCKER_HOST="$candidate" timeout 20 docker version >/dev/null 2>&1; then
      printf '%s' "$candidate"
      return 0
    fi
    echo "docker daemon not reachable at ${candidate}" >&2
  done < <(docker_host_candidates)
  return 1
}

if ! DOCKER_HOST=$(resolve_docker_host); then
  echo "no reachable docker daemon, so the image cannot be built here" >&2
  echo "run scripts/ci-docker-probe.sh in the same job to see what it can see" >&2
  exit 1
fi
export DOCKER_HOST
printf '%s\n' "$DOCKER_HOST" >"$DOCKER_HOST_RECORD" || true
echo "building against ${DOCKER_HOST}"

# No tag anyone can push and no --push. This proves the build, nothing else.
# The publisher on the deploy runner remains the only thing that ships an image.
docker build --pull=false -t sirens-echo:pr-check .
echo "image build succeeded"
