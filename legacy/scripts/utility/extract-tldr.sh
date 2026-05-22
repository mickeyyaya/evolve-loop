#!/usr/bin/env bash
#
# extract-tldr.sh — Extract the ## TLDR section from a markdown file.
#
# Prints content between "## TLDR" and the next "## " heading to stdout.
# Exit 0 if found, 1 if not found.
#
# Usage: bash legacy/scripts/utility/extract-tldr.sh <path/to/file.md>

set -uo pipefail

if [ $# -eq 0 ]; then
    echo "Usage: bash legacy/scripts/utility/extract-tldr.sh <path/to/file.md>" >&2
    exit 1
fi

file="$1"

if [ ! -f "$file" ]; then
    echo "extract-tldr: file not found: $file" >&2
    exit 1
fi

# Extract lines between "## TLDR" and the next "## " heading (exclusive).
# Uses awk — bash 3.2 compatible.
result=$(awk '
    /^## TLDR/ { in_tldr=1; next }
    in_tldr && /^## / { in_tldr=0 }
    in_tldr { print }
' "$file")

if [ -z "$result" ]; then
    echo "extract-tldr: no ## TLDR section found in $file" >&2
    exit 1
fi

printf '%s\n' "$result"
exit 0
