#!/usr/bin/env bash
# AC-ID: cycle-94-002-fast-fail-counter-logic
# AC-source: cycle-94/intent.md acceptance_check #1
# Behavioral predicate: P1 — scripts/dispatch/subagent-run.sh must
# implement the fast-fail retry-exhaustion counter:
#   * Named constant FAST_FAIL_THRESHOLD_S=5
#   * Named constant FAST_FAIL_MAX_CONSECUTIVE=2
#   * Emits ledger entry of kind "retry_exhausted_fastfail" when the
#     counter trips
#   * Trigger pair is `exit_code != 0 AND duration < threshold` (the
#     challenged-premise correction — a genuine fast success must NOT
#     burn the retry budget)
#   * write_ledger_entry accepts an optional `kind` parameter (default
#     "agent_subprocess" preserves backward compatibility)
#
# Rationale: two consecutive dead launches (exit!=0 in <5s) indicate
# structural dispatch failure (sandbox EPERM, binary missing, auth
# error). A third attempt produces the same result at additional cost.
# Structural enforcement at the runner level prevents the orchestrator
# (LLM) from infinitely retrying.
#
# RED until Builder adds the block; GREEN once all four signals present.
# Bash 3.2 compatible.
#
# Exit codes:
#   0 = GREEN
#   1 = RED
set -uo pipefail

REPO_ROOT="${WORKTREE_PATH:-${EVOLVE_PROJECT_ROOT:-$(git rev-parse --show-toplevel 2>/dev/null)}}"
if [ -z "$REPO_ROOT" ]; then
  echo "RED: not inside a git work tree" >&2
  exit 1
fi
cd "$REPO_ROOT" || { echo "RED: cd failed" >&2; exit 1; }

TARGET="scripts/dispatch/subagent-run.sh"
if [ ! -f "$TARGET" ]; then
  echo "RED: $TARGET not found" >&2
  exit 1
fi

# Check 1: named constants for threshold + max consecutive
if ! grep -Eq '^[[:space:]]*FAST_FAIL_THRESHOLD_S=' "$TARGET"; then
  echo "RED: $TARGET missing FAST_FAIL_THRESHOLD_S named constant" >&2
  exit 1
fi
if ! grep -Eq '^[[:space:]]*FAST_FAIL_MAX_CONSECUTIVE=' "$TARGET"; then
  echo "RED: $TARGET missing FAST_FAIL_MAX_CONSECUTIVE named constant" >&2
  exit 1
fi

# Check 2: ledger kind string emitted
if ! grep -Fq 'retry_exhausted_fastfail' "$TARGET"; then
  echo "RED: $TARGET does not emit retry_exhausted_fastfail ledger kind" >&2
  exit 1
fi

# Check 3: write_ledger_entry function must accept a `kind` parameter.
# Match either `local kind=` (parameter assignment) or a $kind reference
# in the jq arg block. Both signals confirm the function was extended.
if ! grep -Eq 'local[[:space:]]+kind=|--arg[[:space:]]+kind' "$TARGET"; then
  echo "RED: $TARGET write_ledger_entry not extended with kind parameter" >&2
  exit 1
fi

# Check 4: trigger pair — duration AND exit_code condition must both
# appear in close proximity (within a ~30-line window) so a stray
# mention doesn't count as a satisfied predicate.
# Strategy: locate the FAST_FAIL_THRESHOLD_S use and confirm a paired
# `cli_exit` / `exit_code` reference nearby.
threshold_line=$(grep -n 'FAST_FAIL_THRESHOLD_S' "$TARGET" \
                 | grep -v '^[0-9]*:[[:space:]]*FAST_FAIL_THRESHOLD_S=' \
                 | head -1 | cut -d: -f1)
if [ -z "$threshold_line" ]; then
  echo "RED: FAST_FAIL_THRESHOLD_S is declared but never used" >&2
  exit 1
fi
start_line=$threshold_line
end_line=$((threshold_line + 30))
window=$(sed -n "${start_line},${end_line}p" "$TARGET")
if ! echo "$window" | grep -Eq 'cli_exit|exit_code'; then
  echo "RED: fast-fail trigger does not pair duration with non-zero exit_code (challenged premise)" >&2
  exit 1
fi

echo "GREEN: fast-fail counter implemented with named thresholds and exit_code+duration pair"
exit 0
