#!/usr/bin/env bash
# Reports what a job container can see of the runner's dind daemon. Temporary,
# for coilyco-gaming/sirens-echo#91. Never fails: the caller wants the report.
set -u

echo "== docker-related environment =="
env | grep -i docker || echo "no docker variables"

echo "== client on PATH =="
command -v docker || echo "no docker client"
docker --version 2>&1 || true

echo "== mounted socket =="
ls -l /var/run/docker.sock 2>&1 || echo "no socket"

echo "== routes, for the bridge gateway =="
ip route 2>&1 || cat /proc/net/route 2>&1 || true

echo "== candidate daemon addresses =="
for host in "${DOCKER_HOST:-}" tcp://localhost:2375 tcp://172.17.0.1:2375 \
  tcp://host.docker.internal:2375 unix:///var/run/docker.sock; do
  [ -n "$host" ] || continue
  echo "-- trying $host"
  DOCKER_HOST="$host" timeout 15 docker version --format '{{.Server.Version}}' 2>&1 ||
    echo "unreachable"
done

echo "== probe complete =="
exit 0
