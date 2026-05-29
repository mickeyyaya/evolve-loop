#!/usr/bin/env bash
# AC-ID: cycle-103-005-subagent-run-allowlist-includes-build-planner
# AC-source: scout-report.md AC-4 (lines 323, 358-361), edit spec lines 172-189
# Behavioral predicate:
#   scripts/dispatch/subagent-run.sh must include "build-planner" in BOTH
#   agent_role allowlists:
#     - cmd_run         (around line 631 per scout-report.md:37)
#     - cmd_dispatch_parallel (around line 1454 per scout-report.md:37)
#
# This predicate is structural: it locates BOTH allowlist regexes (a `^(...)$`
# alternation that includes "scout|tdd-engineer|builder|...") and asserts each
# alternation contains "build-planner".
#
# Mutation spec (cycle-103-005-MUT):
#   Mutant: build-planner in one allowlist but not the other  -> must FAIL.
#   Mutant: build-planner only in a comment line              -> must FAIL.
#   Mutant: typo "build_planner" (underscore)                 -> must FAIL.
#
# Bash 3.2 compatible. No GNU-only flags.
#
# Exit codes:
#   0 = GREEN
#   1 = RED
set -uo pipefail

REPO_ROOT="$(git rev-parse --show-toplevel 2>/dev/null)"
if [ -z "$REPO_ROOT" ]; then
  echo "RED: not inside a git work tree" >&2
  exit 1
fi
cd "$REPO_ROOT" || { echo "RED: cd failed" >&2; exit 1; }

TARGET="scripts/dispatch/subagent-run.sh"

if [ ! -f "$TARGET" ]; then
  echo "RED: $TARGET does not exist" >&2
  exit 1
fi

# Count alternation-style allowlist lines that include 'scout' AND 'tdd-engineer'
# AND 'build-planner' (canonical signature of the role allowlist).
# BSD grep -c exits 1 on zero matches; sanitize via || true and default to 0.
count_match_a=$(grep -cE '\(.*scout.*tdd-engineer.*build-planner.*\)' "$TARGET" 2>/dev/null || true)
count_match_a=${count_match_a:-0}
count_match_b=$(grep -cE '\(.*build-planner.*scout.*tdd-engineer.*\)' "$TARGET" 2>/dev/null || true)
count_match_b=${count_match_b:-0}

# Strip any whitespace (defensive).
count_match_a=$(printf '%s' "$count_match_a" | tr -d '[:space:]')
count_match_b=$(printf '%s' "$count_match_b" | tr -d '[:space:]')

# Final safety -- default to 0 if empty / non-numeric.
case "$count_match_a" in ''|*[!0-9]*) count_match_a=0 ;; esac
case "$count_match_b" in ''|*[!0-9]*) count_match_b=0 ;; esac

total_match=$(( count_match_a + count_match_b ))

if [ "$total_match" -lt 2 ]; then
  echo "RED: $TARGET has fewer than 2 allowlist alternations containing build-planner+scout+tdd-engineer (found $total_match)" >&2
  echo "Expected: both cmd_run and cmd_dispatch_parallel allowlists include 'build-planner'." >&2
  grep -nE 'scout.*tdd-engineer' "$TARGET" >&2 || true
  exit 1
fi

echo "GREEN: $TARGET includes 'build-planner' in $total_match allowlist alternation(s)"
exit 0
