#!/usr/bin/env bash
# Daemon addresses for the image-build job, shared so the probe cannot report on
# an address the build never tried. See docs/sirens-echo-image-build.md.

# Where the build records the address it settled on, for the probe to read. The
# two steps share a container, so a step-temp file crosses between them.
DOCKER_HOST_RECORD="${RUNNER_TEMP:-/tmp}/sirens-echo-docker-host"

# default_gateway reads /proc/net/route, whose addresses are little-endian hex.
default_gateway() {
  local raw
  raw=$(awk '$2 == "00000000" && $8 == "00000000" { print $3; exit }' /proc/net/route 2>/dev/null)
  [ -n "$raw" ] || return 1
  printf '%d.%d.%d.%d' \
    "0x${raw:6:2}" "0x${raw:4:2}" "0x${raw:2:2}" "0x${raw:0:2}"
}

# docker_host_candidates prints one address per line, in the order to try them.
# Derived before hardcoded, so renumbering the bridge does not break the build.
docker_host_candidates() {
  local gateway
  if [ -n "${DOCKER_HOST:-}" ]; then
    printf '%s\n' "$DOCKER_HOST"
  fi
  if gateway=$(default_gateway); then
    printf 'tcp://%s:2375\n' "$gateway"
  fi
  printf 'tcp://172.17.0.1:2375\n'
  printf 'unix:///var/run/docker.sock\n'
}
