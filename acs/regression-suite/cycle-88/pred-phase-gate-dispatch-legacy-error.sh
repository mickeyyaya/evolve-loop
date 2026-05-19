#!/usr/bin/env bash
# AC-ID: cycle-88-phase-gate-dispatch-legacy-error
#
# Verifies Cycle B dispatch contract on scripts/lifecycle/phase-gate.sh:
#   1. Invoking the script with legacy gate name `intent-to-research` exits 2
#      and prints an error referencing the migration.
#   2. Invoking the script with legacy gate name `research-to-discover` exits 2
#      and prints an error referencing the migration.
#   3. Invoking the script with new gate name `intent-to-discover` is RECOGNIZED
#      by the dispatcher (must NOT emit "Unknown gate" — exit code may be
#      non-zero because cycle-state/workspace stubs do not exist in this test,
#      but the dispatcher must route to gate_intent_to_discover).
#
# Behavioral: actually invokes the gate script — does not just grep for case
# branches. Mutants that delete the legacy-error branch (so it falls through to
# "Unknown gate") fail criterion (1)/(2); mutants that omit the new dispatch
# entry fail criterion (3).
set -uo pipefail

REPO_ROOT="${WORKTREE_PATH:-${EVOLVE_PROJECT_ROOT:-$(git rev-parse --show-toplevel 2>/dev/null || pwd)}}"
GATE_FILE="$REPO_ROOT/scripts/lifecycle/phase-gate.sh"

if [ ! -f "$GATE_FILE" ]; then
  echo "RED cycle-88-phase-gate-dispatch-legacy-error: phase-gate.sh missing at $GATE_FILE"
  exit 1
fi

fail=0
errors=""

tmp_workspace=$(mktemp -d)
trap 'rm -rf "$tmp_workspace"' EXIT

# (1) Legacy gate name intent-to-research → exit 2 + error text.
out_legacy1=$(bash "$GATE_FILE" intent-to-research 88 "$tmp_workspace" 2>&1 || true)
rc_legacy1=$(bash "$GATE_FILE" intent-to-research 88 "$tmp_workspace" >/dev/null 2>&1; echo $?)
if [ "$rc_legacy1" != "2" ]; then
  errors="${errors}\n  legacy 'intent-to-research' invocation: expected exit 2, got $rc_legacy1"
  fail=$((fail + 1))
fi
if ! printf '%s' "$out_legacy1" | grep -qiE 'retired|migrat|intent-to-discover'; then
  errors="${errors}\n  legacy 'intent-to-research' invocation: error text missing migration pointer (saw: ${out_legacy1:0:120})"
  fail=$((fail + 1))
fi

# (2) Legacy gate name research-to-discover → exit 2 + error text.
out_legacy2=$(bash "$GATE_FILE" research-to-discover 88 "$tmp_workspace" 2>&1 || true)
rc_legacy2=$(bash "$GATE_FILE" research-to-discover 88 "$tmp_workspace" >/dev/null 2>&1; echo $?)
if [ "$rc_legacy2" != "2" ]; then
  errors="${errors}\n  legacy 'research-to-discover' invocation: expected exit 2, got $rc_legacy2"
  fail=$((fail + 1))
fi
if ! printf '%s' "$out_legacy2" | grep -qiE 'retired|migrat|intent-to-discover'; then
  errors="${errors}\n  legacy 'research-to-discover' invocation: error text missing migration pointer (saw: ${out_legacy2:0:120})"
  fail=$((fail + 1))
fi

# (3) New gate name intent-to-discover → dispatcher must recognize it.
# We do not care whether the gate ultimately passes/fails (it needs a valid
# intent.md in the workspace, which we don't stage here) — we only assert
# the script does NOT emit "Unknown gate: intent-to-discover".
out_new=$(bash "$GATE_FILE" intent-to-discover 88 "$tmp_workspace" 2>&1 || true)
if printf '%s' "$out_new" | grep -qE 'Unknown gate: intent-to-discover'; then
  errors="${errors}\n  new 'intent-to-discover' invocation hit 'Unknown gate' branch — dispatcher missing entry"
  fail=$((fail + 1))
fi

if [ $fail -gt 0 ]; then
  echo "RED cycle-88-phase-gate-dispatch-legacy-error: $fail issue(s)"
  printf "%b\n" "$errors" >&2
  exit 1
fi
echo "GREEN cycle-88-phase-gate-dispatch-legacy-error: legacy gate names emit clear retirement error; new intent-to-discover dispatcher entry present"
exit 0
