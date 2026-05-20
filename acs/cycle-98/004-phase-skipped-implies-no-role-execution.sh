#!/usr/bin/env bash
# AC-ID: cycle-98-004-phase-skipped-implies-no-role-execution
# AC-source: cycle-98/intent.md acceptance_checks[3] ; scout-report.md TASK-98-D
# Behavioral predicate (load-bearing contract for P3 PSMAS):
#
#   FOR every cycle N where the ledger contains an entry with
#       kind == "phase_skipped" AND role == R,
#   THERE MUST BE ZERO entries in the same cycle N with role == R AND
#       kind matching an agent-execution kind (anything other than
#       "phase_skipped"). i.e. the orchestrator either skipped the
#       phase or executed it — never both.
#
# This predicate also exercises the contract against a synthetic fixture
# so it has positive AND negative test discrimination (defeats trivial
# mutation-testing tautologies). Two fixture cases are checked in-line:
#
#   FIXTURE-A (positive): cycle 9000 with one phase_skipped role:retrospective
#                         and zero retrospective agent entries -> contract holds.
#   FIXTURE-B (negative): cycle 9001 with one phase_skipped role:retrospective
#                         and one retrospective agent entry   -> contract violated;
#                         the validator MUST flag this.
#
# Then the validator is applied to the live ledger. Any cycle whose live
# ledger violates the contract turns the predicate RED.
#
# RED until orchestrator implementation respects the contract on any
# cycle that records a phase_skipped entry; GREEN when (a) the validator
# passes both fixture sanity checks AND (b) no live-ledger cycle violates
# the contract.
#
# Bash 3.2 compatible. No GNU-only flags. Uses jq for JSONL parsing.
#
# Exit codes:
#   0 = GREEN (contract holds in fixture A, fails in fixture B as expected,
#              and no live-ledger violations)
#   1 = RED   (validator broken or live ledger violates contract)
set -uo pipefail

REPO_ROOT="$(git rev-parse --show-toplevel 2>/dev/null)"
if [ -z "$REPO_ROOT" ]; then
  echo "RED: not inside a git work tree" >&2
  exit 1
fi
cd "$REPO_ROOT" || { echo "RED: cd failed" >&2; exit 1; }

if ! command -v jq >/dev/null 2>&1; then
  echo "RED: jq required for this predicate (JSONL parsing)" >&2
  exit 1
fi

# Walker: validate the phase_skip contract over a given ledger file.
#   $1 = ledger path
# Echoes "0" on PASS, "1" on FAIL plus a violation summary on stderr.
_validate_contract() {
  local ledger="$1"
  [ -f "$ledger" ] || { echo 0; return 0; }

  # 1) Collect all (cycle, role) pairs that were marked phase_skipped.
  local skipped_pairs
  skipped_pairs="$(jq -r 'select(.kind=="phase_skipped") | "\(.cycle)\t\(.role)"' "$ledger" 2>/dev/null | sort -u)"
  if [ -z "$skipped_pairs" ]; then
    echo 0
    return 0
  fi

  local violations=0
  local violation_log=""
  local line cycle role exec_count
  while IFS=$'\t' read -r cycle role; do
    [ -z "$cycle" ] && continue
    # 2) Count agent-execution entries (kind != phase_skipped) for the
    #    same cycle AND role. If non-zero, the contract is violated.
    exec_count="$(jq -r --argjson c "$cycle" --arg r "$role" '
      select(.cycle == $c)
      | select(.role == $r)
      | select(.kind != "phase_skipped")
      | .role' "$ledger" 2>/dev/null | wc -l | tr -d ' ')"
    if [ "${exec_count:-0}" -ne 0 ]; then
      violations=$(( violations + 1 ))
      violation_log="$violation_log  - cycle=$cycle role=$role had $exec_count executing entries despite phase_skipped"$'\n'
    fi
  done <<< "$skipped_pairs"

  if [ "$violations" -ne 0 ]; then
    printf '%s' "$violation_log" >&2
    echo 1
    return 0
  fi
  echo 0
}

# ── Fixture A: contract holds (positive case) ──
TMPDIR_FIX="$(mktemp -d -t cycle98-acs004.XXXXXX)" || { echo "RED: mktemp failed" >&2; exit 1; }
trap 'rm -rf "$TMPDIR_FIX"' EXIT

FIX_A="$TMPDIR_FIX/ledger-A.jsonl"
cat > "$FIX_A" <<'EOF'
{"cycle":9000,"role":"intent","kind":"agent_subprocess"}
{"cycle":9000,"role":"scout","kind":"agent_subprocess"}
{"cycle":9000,"role":"triage","kind":"agent_subprocess"}
{"cycle":9000,"role":"builder","kind":"agent_subprocess"}
{"cycle":9000,"role":"auditor","kind":"agent_subprocess"}
{"cycle":9000,"role":"retrospective","kind":"phase_skipped","reason":"triage_phase_skip","psmas_flag":1}
EOF

if [ "$(_validate_contract "$FIX_A")" != "0" ]; then
  echo "RED: fixture-A (positive) failed validation — validator is broken" >&2
  exit 1
fi

# ── Fixture B: contract violated (negative case) — validator must flag ──
FIX_B="$TMPDIR_FIX/ledger-B.jsonl"
cat > "$FIX_B" <<'EOF'
{"cycle":9001,"role":"intent","kind":"agent_subprocess"}
{"cycle":9001,"role":"retrospective","kind":"phase_skipped","reason":"triage_phase_skip","psmas_flag":1}
{"cycle":9001,"role":"retrospective","kind":"agent_subprocess"}
EOF

if [ "$(_validate_contract "$FIX_B" 2>/dev/null)" != "1" ]; then
  echo "RED: fixture-B (negative) was not flagged — validator fails to detect contract violation" >&2
  exit 1
fi

# ── Live ledger: contract must hold for every cycle that emitted a skip ──
# Resolution order: EVOLVE_PROJECT_ROOT env (set by dispatcher / resolve-roots.sh),
# else repo toplevel, else absent (skip live-ledger check, fixtures suffice).
LIVE_LEDGER=""
if [ -n "${EVOLVE_PROJECT_ROOT:-}" ] && [ -f "$EVOLVE_PROJECT_ROOT/.evolve/ledger.jsonl" ]; then
  LIVE_LEDGER="$EVOLVE_PROJECT_ROOT/.evolve/ledger.jsonl"
elif [ -f "$REPO_ROOT/.evolve/ledger.jsonl" ]; then
  LIVE_LEDGER="$REPO_ROOT/.evolve/ledger.jsonl"
fi

live_scope="(skipped — no live ledger reachable)"
if [ -n "$LIVE_LEDGER" ]; then
  live_status="$(_validate_contract "$LIVE_LEDGER" 2>&1)"
  # The validator echoes 0 or 1 as its last word; capture it.
  live_rc="$(printf '%s' "$live_status" | tail -n 1)"
  if [ "$live_rc" = "1" ]; then
    echo "RED: live ledger violates phase_skipped contract" >&2
    printf '%s\n' "$live_status" >&2
    exit 1
  fi
  live_scope="(live: $LIVE_LEDGER)"
fi

echo "GREEN: phase_skipped⇒no-role-execution contract validated on fixture-A (pass) + fixture-B (correctly flagged) + live $live_scope"
exit 0
