#!/usr/bin/env bash
# AC-ID: cycle-98-005-default-off-no-phase-skipped-baseline
# AC-source: cycle-98/intent.md acceptance_checks[4]
# Behavioral predicate (regression / default-off guarantee):
#   When EVOLVE_PSMAS_SKIP is unset or 0, NO kind:phase_skipped entries
#   may appear in the ledger. This protects the byte-identical-to-today
#   guarantee for every operator who does not opt into the flag.
#
# Two cohort checks are applied:
#
#   (a) The most recent shipped cycle that ran with EVOLVE_PSMAS_SKIP
#       unset MUST contain zero kind:phase_skipped entries. We use
#       cycle-97 as the canonical baseline (last shipped roadmap cycle
#       before P3 foundation).
#   (b) The current cycle (resolved from state.json:lastCycleNumber when
#       EVOLVE_PSMAS_SKIP is not set in the predicate's own env) also
#       MUST contain zero kind:phase_skipped entries.
#
# Synthetic-fixture sanity: a tiny in-line case demonstrates that the
# detection logic catches a violation if one is injected. This blunts
# trivial mutation-test tautologies.
#
# RED if any cohort-(a) phase_skipped entry exists, OR if cohort-(b)
# shows a phase_skipped entry while the predicate's env has the flag
# unset. GREEN otherwise.
#
# Bash 3.2 compatible. No GNU-only flags.
#
# Exit codes:
#   0 = GREEN (no default-off cycle emitted phase_skipped)
#   1 = RED   (default-off cycle emitted phase_skipped — invariant broken)
set -uo pipefail

REPO_ROOT="$(git rev-parse --show-toplevel 2>/dev/null)"
if [ -z "$REPO_ROOT" ]; then
  echo "RED: not inside a git work tree" >&2
  exit 1
fi
cd "$REPO_ROOT" || { echo "RED: cd failed" >&2; exit 1; }

if ! command -v jq >/dev/null 2>&1; then
  echo "RED: jq required" >&2
  exit 1
fi

# Resolve ledger via EVOLVE_PROJECT_ROOT (dispatcher) → repo toplevel.
LEDGER=""
STATE=""
if [ -n "${EVOLVE_PROJECT_ROOT:-}" ] && [ -f "$EVOLVE_PROJECT_ROOT/.evolve/ledger.jsonl" ]; then
  LEDGER="$EVOLVE_PROJECT_ROOT/.evolve/ledger.jsonl"
  STATE="$EVOLVE_PROJECT_ROOT/.evolve/state.json"
elif [ -f "$REPO_ROOT/.evolve/ledger.jsonl" ]; then
  LEDGER="$REPO_ROOT/.evolve/ledger.jsonl"
  STATE="$REPO_ROOT/.evolve/state.json"
fi

# ── Synthetic detector sanity ────────────────────────────────────────
TMPDIR_FIX="$(mktemp -d -t cycle98-acs005.XXXXXX)" || { echo "RED: mktemp failed" >&2; exit 1; }
trap 'rm -rf "$TMPDIR_FIX"' EXIT

FIX="$TMPDIR_FIX/baseline-violation.jsonl"
cat > "$FIX" <<'EOF'
{"cycle":9100,"role":"retrospective","kind":"phase_skipped"}
EOF

count_fix="$(jq -r --argjson c 9100 'select(.cycle==$c and .kind=="phase_skipped") | .role' "$FIX" 2>/dev/null | wc -l | tr -d ' ')"
if [ "${count_fix:-0}" -eq 0 ]; then
  echo "RED: detector failed synthetic sanity (could not see injected phase_skipped)" >&2
  exit 1
fi

# ── Cohorts (a) baseline cycle-97 + (b) current cycle (when flag-unset) ──
fail_count=0
fail_log=""
checked_scope="fixture-only"

if [ -n "$LEDGER" ]; then
  checked_scope="ledger=$LEDGER"

  baseline_cycle=97
  baseline_skipped="$(jq -r --argjson c "$baseline_cycle" '
    select(.cycle == $c and .kind == "phase_skipped") | .role' "$LEDGER" 2>/dev/null | wc -l | tr -d ' ')"
  if [ "${baseline_skipped:-0}" -ne 0 ]; then
    fail_count=$(( fail_count + 1 ))
    fail_log="$fail_log  - cycle-$baseline_cycle (default-off baseline) has $baseline_skipped phase_skipped entries"$'\n'
  fi

  if [ -z "${EVOLVE_PSMAS_SKIP:-}" ] || [ "${EVOLVE_PSMAS_SKIP:-0}" = "0" ]; then
    current_cycle=""
    if [ -n "$STATE" ] && [ -f "$STATE" ]; then
      current_cycle="$(jq -r '.lastCycleNumber // empty' "$STATE" 2>/dev/null || true)"
    fi
    if [ -n "$current_cycle" ]; then
      current_skipped="$(jq -r --argjson c "$current_cycle" '
        select(.cycle == $c and .kind == "phase_skipped") | .role' "$LEDGER" 2>/dev/null | wc -l | tr -d ' ')"
      if [ "${current_skipped:-0}" -ne 0 ]; then
        fail_count=$(( fail_count + 1 ))
        fail_log="$fail_log  - current cycle-$current_cycle ran flag-unset yet emitted $current_skipped phase_skipped entries"$'\n'
      fi
    fi
  fi
fi

if [ "$fail_count" -ne 0 ]; then
  printf 'RED: default-off baseline invariant broken (%s issue[s]):\n' "$fail_count" >&2
  printf '%s' "$fail_log" >&2
  exit 1
fi

echo "GREEN: no kind:phase_skipped entries in default-off cohorts ($checked_scope)"
exit 0
