#!/usr/bin/env bash
# AC-ID: cycle-94-003-orchestrator-fast-fail-stop-criterion
# AC-source: cycle-94/intent.md acceptance_check #2
# Behavioral predicate: P1 — agents/evolve-orchestrator.md must contain
# the fast-fail stop criterion that instructs the orchestrator NOT to
# retry a phase after a retry_exhausted_fastfail ledger entry exists,
# and to emit verdict BLOCKED-FAST-FAIL.
#
# Rationale: the runner's fast-fail block (predicate 002) writes the
# ledger entry, but the orchestrator (LLM) is the actor that decides
# whether to invoke subagent-run.sh a third time. The stop-criterion
# paragraph closes that loop by giving the LLM a deterministic rule.
#
# RED until Builder adds the section; GREEN once both signals present.
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

TARGET="agents/evolve-orchestrator.md"
if [ ! -f "$TARGET" ]; then
  echo "RED: $TARGET not found" >&2
  exit 1
fi

# Check 1: orchestrator references the retry_exhausted_fastfail ledger kind
if ! grep -Fq 'retry_exhausted_fastfail' "$TARGET"; then
  echo "RED: $TARGET does not reference retry_exhausted_fastfail ledger kind" >&2
  exit 1
fi

# Check 2: the verdict label BLOCKED-FAST-FAIL appears (binds the
# orchestrator's exit path to a named verdict)
if ! grep -Fq 'BLOCKED-FAST-FAIL' "$TARGET"; then
  echo "RED: $TARGET does not emit BLOCKED-FAST-FAIL verdict" >&2
  exit 1
fi

# Check 3: instruction is anchored under a STOP / abort / fast-fail
# heading (so the rule isn't buried in unrelated prose). Search a
# 25-line window around the retry_exhausted_fastfail mention.
match_line=$(grep -n 'retry_exhausted_fastfail' "$TARGET" | head -1 | cut -d: -f1)
if [ -z "$match_line" ]; then
  echo "RED: retry_exhausted_fastfail match-line unresolved" >&2
  exit 1
fi
window_start=$((match_line - 25))
[ "$window_start" -lt 1 ] && window_start=1
window_end=$((match_line + 25))
window=$(sed -n "${window_start},${window_end}p" "$TARGET")
if ! echo "$window" | grep -Eiq 'stop|abort|fast.?fail|do[[:space:]]+not[[:space:]]+retry'; then
  echo "RED: retry_exhausted_fastfail mention not anchored under stop/abort/fast-fail context" >&2
  exit 1
fi

echo "GREEN: orchestrator carries fast-fail stop criterion + BLOCKED-FAST-FAIL verdict"
exit 0
