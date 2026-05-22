#!/usr/bin/env bash
#
# lint-markdown-structure.sh — WARN-only linter for evolve-loop markdown schema.
#
# Checks: frontmatter present, TLDR section present, TOC present (>100 lines),
# imperative voice heuristic for H2 headings.
#
# Exit: always 0 (WARN-only, no gate integration this cycle).
# Usage: bash scripts/utility/lint-markdown-structure.sh <path> [<path>...]
#   paths may be .md files or directories (recursed for *.md)

set -uo pipefail

WARN=0
PASS=0
SKIP=0

warn() { echo "WARN  $1: $2" >&2; WARN=$((WARN + 1)); }
pass() { PASS=$((PASS + 1)); }
skip() { SKIP=$((SKIP + 1)); }

# Non-imperative words that typically start gerund/noun headings.
# Bash 3.2 compatible: no associative arrays, simple case match.
is_non_imperative_h2() {
    local heading="$1"
    # Strip leading "## " and lowercase via tr
    local word
    word=$(printf '%s\n' "$heading" | sed 's/^#* *//' | awk '{print $1}' | tr '[:upper:]' '[:lower:]')
    case "$word" in
        adding|running|using|configuring|setting|getting|building|creating|removing|updating|testing|checking|enabling|disabling|managing|implementing|defining|describing|specifying|understanding|overview|background|introduction|about|notes|summary|conclusion|details|behavior|behaviours|behaviors|architecture|design|motivation|rationale|context|background|history|status|glossary|appendix|faq|troubleshooting|changelog)
            return 0
            ;;
    esac
    return 1
}

lint_file() {
    local file="$1"

    # Skip non-markdown
    case "$file" in
        *.md) ;;
        *) skip; return ;;
    esac

    [ -f "$file" ] || { skip; return; }

    local line_count
    line_count=$(wc -l < "$file" | tr -d ' ')

    # Check 1: frontmatter present (file must start with ---)
    local first_line
    first_line=$(head -1 "$file" 2>/dev/null || echo "")
    if [ "$first_line" != "---" ]; then
        warn "$file" "missing YAML frontmatter (file does not start with ---)"
    fi

    # Check 2: TLDR section present
    if ! grep -q "^## TLDR" "$file" 2>/dev/null; then
        warn "$file" "missing ## TLDR section"
    fi

    # Check 3: TOC required if >100 lines
    if [ "$line_count" -gt 100 ]; then
        if ! grep -q "^## Table of Contents" "$file" 2>/dev/null; then
            warn "$file" "file has $line_count lines but no ## Table of Contents section"
        fi
    fi

    # Check 4: imperative voice heuristic — H2 headings
    while IFS= read -r hline; do
        if is_non_imperative_h2 "$hline"; then
            warn "$file" "H2 heading may not use imperative voice: '$hline'"
        fi
    done < <(grep "^## " "$file" 2>/dev/null | grep -v "^## TLDR" | grep -v "^## Table of Contents" | grep -v "^## References" || true)

    pass
}

collect_files() {
    local path="$1"
    if [ -d "$path" ]; then
        # Recurse: find *.md files. bash 3.2 compatible with while+read.
        while IFS= read -r f; do
            lint_file "$f"
        done < <(find "$path" -name "*.md" -type f 2>/dev/null | sort)
    elif [ -f "$path" ]; then
        lint_file "$path"
    else
        echo "SKIP  $path: not found" >&2
        SKIP=$((SKIP + 1))
    fi
}

if [ $# -eq 0 ]; then
    echo "Usage: bash scripts/utility/lint-markdown-structure.sh <path> [<path>...]" >&2
    exit 0
fi

for arg in "$@"; do
    collect_files "$arg"
done

echo ""
echo "lint-markdown-structure: $WARN warn(s), $PASS passed, $SKIP skipped"

# Always exit 0 — WARN-only mode.
exit 0
