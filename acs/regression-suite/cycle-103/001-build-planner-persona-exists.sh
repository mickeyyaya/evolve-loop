#!/usr/bin/env bash
# AC-ID: cycle-103-001-build-planner-persona-exists
# AC-source: scout-report.md AC-1 (lines 320, 337-340)
# Behavioral predicate:
#   agents/evolve-build-planner.md must exist with valid YAML frontmatter
#   containing required fields: name, model (tier-1 or opus), tools.
#
# Mutation spec (cycle-103-001-MUT):
#   Mutant: file present but `name:` value != "evolve-build-planner" -> must FAIL.
#   Mutant: file present but `model:` missing -> must FAIL.
#   Mutant: file present but `tools:` missing -> must FAIL.
#   Mutant: file absent -> must FAIL.
#
# Bash 3.2 compatible. No GNU-only flags. No declare -A, no mapfile.
#
# Exit codes:
#   0 = GREEN (predicate satisfied)
#   1 = RED   (predicate violated)
set -uo pipefail

REPO_ROOT="$(git rev-parse --show-toplevel 2>/dev/null)"
if [ -z "$REPO_ROOT" ]; then
  echo "RED: not inside a git work tree" >&2
  exit 1
fi
cd "$REPO_ROOT" || { echo "RED: cd failed" >&2; exit 1; }

PERSONA="agents/evolve-build-planner.md"

if [ ! -f "$PERSONA" ]; then
  echo "RED: $PERSONA does not exist" >&2
  exit 1
fi

# Extract YAML frontmatter (between leading '---' and the next '---').
# Use awk for bash 3.2 portability (no mapfile, no associative arrays).
FRONTMATTER="$(awk '
  /^---[[:space:]]*$/ {
    if (seen == 0) { seen = 1; next }
    else { exit }
  }
  seen == 1 { print }
' "$PERSONA")"

if [ -z "$FRONTMATTER" ]; then
  echo "RED: $PERSONA has no YAML frontmatter (expected leading ---)" >&2
  exit 1
fi

# Check name: evolve-build-planner
if ! printf '%s\n' "$FRONTMATTER" | grep -Eq '^name:[[:space:]]+evolve-build-planner[[:space:]]*$'; then
  echo "RED: $PERSONA frontmatter missing 'name: evolve-build-planner'" >&2
  printf '%s\n' "$FRONTMATTER" | grep -E '^name:' >&2 || true
  exit 1
fi

# Check model: (must contain tier-1 OR opus)
if ! printf '%s\n' "$FRONTMATTER" | grep -Eq '^model:[[:space:]]+(tier-1|opus)([[:space:]].*)?$'; then
  echo "RED: $PERSONA frontmatter missing 'model: tier-1' or 'model: opus'" >&2
  printf '%s\n' "$FRONTMATTER" | grep -E '^model:' >&2 || true
  exit 1
fi

# Check tools:
if ! printf '%s\n' "$FRONTMATTER" | grep -Eq '^tools:[[:space:]]'; then
  echo "RED: $PERSONA frontmatter missing 'tools:' field" >&2
  exit 1
fi

echo "GREEN: $PERSONA exists with valid frontmatter (name, model, tools)"
exit 0
