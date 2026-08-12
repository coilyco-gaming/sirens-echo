#!/usr/bin/env bash
# Stage the admitted composed bodies beneath agent/compose/, then compose one
# bundle per role. A declaration's paths resolve relative to its own directory,
# so the bodies have to sit next to it. See docs/sirens-echo-compose.md.
set -euo pipefail

catalog=${1:?usage: stage-compose-sources.sh <agentic-os checkout> <bundle out dir>}
bundles=${2:?usage: stage-compose-sources.sh <agentic-os checkout> <bundle out dir>}
compose_dir=agent/compose
staged=$compose_dir/skills

rm -rf "$staged"
mkdir -p "$staged"

# Only names the declaration admits are staged, so a body outside the reviewed
# set cannot reach the bundle even if it is present in the catalogue.
names=$(grep -oE '^[[:space:]]*skill "[^"]+"' "$compose_dir/aos-public.kdl" | sed 's/.*"\(.*\)"/\1/')
for name in $names; do
    source_dir="$catalog/.agents/composed/$name"
    if [ ! -f "$source_dir/COMPOSED.md" ]; then
        echo "stage-compose-sources: $name is not in the catalogue at $catalog" >&2
        exit 1
    fi
    mkdir -p "$staged/$name"
    # agent-compose expects SKILL.md at a declared skill path.
    cp "$source_dir/COMPOSED.md" "$staged/$name/SKILL.md"
    if [ -d "$source_dir/references" ]; then
        cp -R "$source_dir/references" "$staged/$name/references"
    fi
done
echo "staged $(echo "$names" | wc -w | tr -d ' ') composed sources from $catalog"

mkdir -p "$bundles"
for role in creator director engineer qa ops design ai; do
    out=$bundles/$role
    rm -rf "$out"
    mkdir -p "$out"
    sed "s/^    role \".*\"$/    role \"$role\"/" "$compose_dir/request.kdl" > "$compose_dir/request.$role.kdl"
    ( cd "$compose_dir" && agent-compose compose "request.$role.kdl" --out "$OLDPWD/$out" >/dev/null )
    rm -f "$compose_dir/request.$role.kdl"
    tree=$(find "$out" -mindepth 1 -maxdepth 1 -type d | head -1)
    # The materializer names the tree by content hash; flatten it so the role
    # slug alone selects a bundle at runtime.
    mv "$tree"/* "$out"/ && rmdir "$tree"
    agent-compose verify "$out"
done
