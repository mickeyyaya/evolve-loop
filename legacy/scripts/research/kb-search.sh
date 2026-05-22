#!/usr/bin/env bash
# legacy/scripts/research/kb-search.sh — Local knowledge-base search.
#
# Uses ripgrep (rg) if available, falls back to grep -rE.
# Output: file:line:context hits, capped at 2048 bytes.
#
# Usage:
#   bash legacy/scripts/research/kb-search.sh <pattern>
#
# Env:
#   EVOLVE_KB_SEARCH_PATHS — colon-separated KB roots (default: knowledge-base/research/)

set -uo pipefail

PATTERN="${1:-}"
if [ -z "$PATTERN" ]; then
  printf 'Usage: kb-search.sh <pattern>\n' >&2
  exit 2
fi

# Resolve default KB root relative to this script's repo root
__self_dir="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
REPO_ROOT="$(cd "$__self_dir/../../.." && pwd)"
DEFAULT_KB="$REPO_ROOT/knowledge-base/research"

# Split EVOLVE_KB_SEARCH_PATHS by ':' (bash 3.2 compatible via set --)
_paths="${EVOLVE_KB_SEARCH_PATHS:-$DEFAULT_KB}"
OLD_IFS="$IFS"
IFS=":"
set -- $_paths
IFS="$OLD_IFS"

OUTPUT=""
FOUND=0

for root; do
  [ -d "$root" ] || continue

  if command -v rg >/dev/null 2>&1; then
    hits=$(rg --color=never -n --max-count 10 "$PATTERN" "$root" 2>/dev/null || true)
  else
    hits=$(grep -rEn "$PATTERN" "$root" 2>/dev/null || true)
  fi

  if [ -n "$hits" ]; then
    OUTPUT="${OUTPUT}${hits}"
    OUTPUT="${OUTPUT}
"
    FOUND=1
  fi
done

if [ "$FOUND" = "0" ]; then
  printf 'No matches found for: %s\n' "$PATTERN"
  exit 0
fi

# Cap output at 2048 bytes
printf '%s' "$OUTPUT" | head -c 2048
exit 0
