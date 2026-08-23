#!/usr/bin/env bash
set -euo pipefail

# Makes `derived: true` mean something instead of asserting it.
# Format and the prose gap: docs/sirens-echo-attributes.md

FILE=eval/attributes.yaml

case "${1:-list}" in
  list)
    # Two challenges per attribute, because the pair is the scoring unit.
    yq -r '.attributes[] | "\(.id)\n  rule    : \(.rule)\n  inside  : \(.inside)\n  outside : \(.outside)"' "$FILE"
    total=$(yq -r '.attributes | length' "$FILE")
    echo
    echo "$total attributes, $((total * 2)) challenges"
    ;;
  check)
    status=0
    count=$(yq -r '.attributes | length' "$FILE")
    for i in $(seq 0 $((count - 1))); do
      id=$(yq -r ".attributes[$i].id" "$FILE")
      origin=$(yq -r ".attributes[$i].origin" "$FILE")
      derived=$(yq -r ".attributes[$i].derived" "$FILE")
      path=${origin%%#*}
      frag=""
      [ "$origin" != "$path" ] && frag=${origin#*#}
      if [ ! -e "$path" ]; then
        echo "boundaries: $id names $path, which does not exist" >&2
        status=1
        continue
      fi
      # A prose boundary names where the clause lives, not checkable wording.
      if [ "$derived" = "true" ] && [ -n "$frag" ] && ! grep -q -- "$frag" "$path"; then
        echo "boundaries: $id claims $origin, but $frag is not in $path" >&2
        status=1
      fi
      for key in rule inside outside; do
        value=$(yq -r ".attributes[$i].$key // \"\"" "$FILE")
        if [ -z "$value" ]; then
          echo "boundaries: $id has no $key, so it cannot produce a pair" >&2
          status=1
        fi
      done
    done
    duplicates=$(yq -r '.attributes[].id' "$FILE" | sort | uniq -d)
    if [ -n "$duplicates" ]; then
      echo "boundaries: duplicate ids:" >&2
      printf '  %s\n' $duplicates >&2
      status=1
    fi
    if [ "$status" -eq 0 ]; then
      derived_count=$(yq -r '[.attributes[] | select(.derived == true)] | length' "$FILE")
      prose_count=$(yq -r '[.attributes[] | select(.derived == false)] | length' "$FILE")
      echo "boundaries: $count declared, $derived_count derived from source, $prose_count prose"
      # Not a failure. Recorded rather than hidden.
      [ "$prose_count" -gt 0 ] && echo "boundaries: $prose_count clause(s) cannot be drift-checked"
    fi
    exit "$status"
    ;;
  *)
    echo "usage: attributes.sh [list|check]" >&2
    exit 2
    ;;
esac
