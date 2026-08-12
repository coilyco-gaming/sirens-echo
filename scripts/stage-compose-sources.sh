#!/usr/bin/env bash
# Bake one verified bundle per role declared in agent/compose/roles.kdl.
# sirens-echo-compose expands that graph against the pinned catalogue and writes
# the declaration agent-compose consumes. See docs/sirens-echo-compose.md.
set -euo pipefail

catalog=${1:?usage: stage-compose-sources.sh <agentic-os checkout> <bundle out dir>}
bundles=${2:?usage: stage-compose-sources.sh <agentic-os checkout> <bundle out dir>}
compose_dir=agent/compose

mkdir -p "$bundles"
# Composition runs from the declaration's directory, so this must be absolute.
bundles=$(cd "$bundles" && pwd)

# A scratch HOME with a minimal config keeps this hermetic. Without the config
# the run converges a whole home tree instead of materializing a bundle.
scratch_home=$(mktemp -d)
trap 'rm -rf "$scratch_home" "$compose_dir/skills" "$compose_dir"/aos-public.kdl "$compose_dir"/request.*.kdl' EXIT
mkdir -p "$scratch_home/.agent-compose" "$scratch_home/.claude"
cat > "$scratch_home/.agent-compose/agent-compose.yaml" <<YAML
load_points:
  claude: $scratch_home/.claude/CLAUDE.md
YAML

generator=$scratch_home/sirens-echo-compose
go build -o "$generator" ./cmd/sirens-echo-compose

roles=$(grep -oE '^[[:space:]]*role[[:space:]]+"[^"]+"' "$compose_dir/roles.kdl" | sed 's/.*"\(.*\)"/\1/')
if [ -z "$roles" ]; then
    echo "stage-compose-sources: roles.kdl declares no role" >&2
    exit 1
fi

for role in $roles; do
    "$generator" --catalog "$catalog" --role "$role" --compose-dir "$compose_dir"
    out=$bundles/$role
    rm -rf "$out"
    mkdir -p "$out"
    sed "s/^    role \".*\"$/    role \"$role\"/" "$compose_dir/request.kdl" > "$compose_dir/request.$role.kdl"
    ( cd "$compose_dir" && HOME=$scratch_home agent-compose compose "request.$role.kdl" --out "$out" >/dev/null )
    # The materializer names the tree by content hash; flatten it so the role
    # slug alone selects a bundle at runtime.
    tree=$(find "$out" -mindepth 1 -maxdepth 1 -type d | head -1)
    mv "$tree"/* "$out"/ && rmdir "$tree"
    HOME=$scratch_home agent-compose verify "$out"
done
