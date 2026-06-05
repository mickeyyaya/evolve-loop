#!/usr/bin/env bash
# ACS cycle-217 / Task-1 AC3+AC4+AC5 — bugfix-pair contract fields per
# micro-phase-catalog.md §3 Wave 1:
#   - fault-localization routes on scout.goal_type == bugfix
#   - bug-reproduction carries the FAIL_TO_PASS gate (fail_if_signal
#     repro.failing == "==false") — a repro that doesn't fail is a failed phase
#   - both are optional:true and writes_source:false
#
# Mixed predicate: the behavioral anchor is `evolve phases validate` (the same
# JSON must load + validate through the real Go path); jq then asserts the
# field-level contract on the exact bytes the loader reads.
set -uo pipefail
TOP=$(git rev-parse --show-toplevel) || { echo "RED: not a git repo" >&2; exit 1; }
cd "$TOP" || { echo "RED: cannot cd to repo root" >&2; exit 1; }

BIN="${EVOLVE_GO_BIN:-$TOP/go/bin/evolve}"
[ -x "$BIN" ] || BIN="$TOP/go/evolve"
[ -x "$BIN" ] || { echo "RED: evolve binary not found" >&2; exit 1; }

FL=".evolve/phases/fault-localization/phase.json"
RB=".evolve/phases/bug-reproduction/phase.json"
[ -f "$FL" ] || { echo "RED: $FL missing" >&2; exit 1; }
[ -f "$RB" ] || { echo "RED: $RB missing" >&2; exit 1; }

# Behavioral anchor: both specs must pass real validation first.
for p in fault-localization bug-reproduction; do
  EVOLVE_PROJECT_ROOT="$TOP" "$BIN" phases validate "$p" >/dev/null 2>&1 \
    || { echo "RED: phases validate $p failed — field checks below would be moot" >&2; exit 1; }
done

# AC3 — bugfix routing trigger.
jq -e '.routing.insert_when[]? | select(.field=="scout.goal_type" and (.op=="==" or .op=="eq") and .value=="bugfix")' \
  "$FL" >/dev/null 2>&1 \
  || { echo "RED: fault-localization lacks insert_when scout.goal_type==bugfix" >&2; exit 1; }

# AC4 — FAIL_TO_PASS gate on the reproduction.
jq -e '.classify.fail_if_signal["repro.failing"] == "==false"' "$RB" >/dev/null 2>&1 \
  || { echo "RED: bug-reproduction lacks fail_if_signal repro.failing==\"==false\"" >&2; exit 1; }

# AC5 — optional:true + writes_source:false on both.
for f in "$FL" "$RB"; do
  [ "$(jq -r '.optional' "$f")" = "true" ] \
    || { echo "RED: $f optional != true" >&2; exit 1; }
  [ "$(jq -r '.writes_source // false' "$f")" = "false" ] \
    || { echo "RED: $f writes_source != false" >&2; exit 1; }
done

echo "GREEN: bugfix-pair contract fields match catalog §3 (routing, FAIL_TO_PASS gate, optional/non-writing)" >&2
exit 0
