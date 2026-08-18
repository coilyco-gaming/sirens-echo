#!/usr/bin/env bash
# Local `just image` build. Resolves the catalogue head the same way CI does, so a
# workstation image is not quietly baked from a frozen clone layer.
set -euo pipefail

script_dir=$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)
# shellcheck source=scripts/lib/catalog-head.sh
. "${script_dir}/lib/catalog-head.sh"

catalog_ref="${AOS_CATALOG_REF:-main}"
catalog_sha=$(catalog_head "${catalog_ref}")
echo "baking catalogue ${catalog_ref} at ${catalog_sha}"

docker build \
  --build-arg AOS_CATALOG_REF="${catalog_ref}" \
  --build-arg AOS_CATALOG_HEAD="${catalog_sha}" \
  -t sirens-echo:dev "$@" .
