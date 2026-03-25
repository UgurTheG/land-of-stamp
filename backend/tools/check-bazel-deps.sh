#!/usr/bin/env bash
# check-bazel-deps.sh — warn when bazel_dep versions in MODULE.bazel
# fall behind the Bazel Central Registry (BCR), or don't exist at all.
#
# Usage:  ./tools/check-bazel-deps.sh [MODULE.bazel]
#
# Exit codes:
#   0  all deps up to date and valid
#   1  one or more deps outdated or invalid

set -euo pipefail

MODULE_FILE="${1:-MODULE.bazel}"
BCR="https://bcr.bazel.build/modules"

# Temp file to signal failure across the subshell boundary
STATUS_FILE=$(mktemp)
echo 0 > "$STATUS_FILE"

# Parse bazel_dep(name = "…", version = "…") lines
grep -oP 'bazel_dep\(name\s*=\s*"\K[^"]+"\s*,\s*version\s*=\s*"[^"]+' "$MODULE_FILE" |
  sed 's/"\s*,\s*version\s*=\s*"/ /' |
while read -r name current; do
  # Fetch all known versions from BCR
  json=$(curl -sf "$BCR/$name/metadata.json" 2>/dev/null) || true

  if [ -z "$json" ]; then
    echo "⚠️  $name: could not fetch metadata from BCR"
    continue
  fi

  # Check existence + get latest in one python call
  eval "$(echo "$json" | python3 -c "
import sys, json
d = json.load(sys.stdin)
vs = d.get('versions', [])
current = '$current'
latest = vs[-1] if vs else ''
exists = current in vs
print(f'latest={latest}')
print(f'exists={str(exists).lower()}')
")"

  if [ "$exists" = "false" ]; then
    echo "❌  $name $current does NOT exist in BCR (latest: $latest)"
    echo 1 > "$STATUS_FILE"
  elif [ "$current" = "$latest" ]; then
    echo "✅  $name $current (up to date)"
  else
    echo "❌  $name $current → $latest available"
    echo 1 > "$STATUS_FILE"
  fi
done

rc=$(cat "$STATUS_FILE")
rm -f "$STATUS_FILE"

if [ "$rc" != "0" ]; then
  echo ""
  echo "::error::One or more bazel_dep versions are outdated or invalid. Update MODULE.bazel and run: bazel mod deps --lockfile_mode=update"
  exit 1
fi
