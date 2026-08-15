#!/usr/bin/env bash
# Run the Postgres store's SQL tests and fail if any of them skipped.
#
# `go test` prints ok and exits 0 for a skip, so a service container that never
# came up, or a DSN that never reached the step, would leave this green having
# executed no SQL at all. That is the shape docs/sirens-echo-test-skips.md is
# about, arriving in the one step whose whole job is to run what the suite skips.
set -euo pipefail

if [ "$#" -ne 0 ]; then
  echo "usage: ci-job-store-sql.sh" >&2
  exit 2
fi

if [ -z "${SIRENS_ECHO_TEST_JOB_STORE_DSN:-}" ]; then
  echo "ci-job-store-sql: SIRENS_ECHO_TEST_JOB_STORE_DSN is unset" >&2
  exit 1
fi

output=$(go test -v -count=1 -run TestThePostgresStore ./internal/community/ 2>&1) || {
  echo "$output"
  exit 1
}
echo "$output"

skipped=$(printf '%s\n' "$output" | sed -n 's/^ *--- SKIP: \([A-Za-z0-9_]*\).*/\1/p')
if [ -n "$skipped" ]; then
  echo "ci-job-store-sql: these tests skipped instead of running the SQL:" >&2
  printf '%s\n' "$skipped" >&2
  exit 1
fi

ran=$(printf '%s\n' "$output" | grep -c '^--- PASS: TestThePostgresStore' || true)
if [ "$ran" -eq 0 ]; then
  echo "ci-job-store-sql: no job store test reported a pass" >&2
  exit 1
fi
echo "ci-job-store-sql: ${ran} job store SQL test(s) ran against Postgres"
