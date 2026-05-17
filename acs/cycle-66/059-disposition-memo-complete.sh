#!/usr/bin/env bash
# ACS predicate 059 — cycle 66
# Verifies the cycle-66 backlog disposition memo records explicit fate for
# all 22 enumerated inbox items, with each row classified as one of
# {resolved-shipped, partial-resolved, deferred, dropped}.
#
# AC-ID: cycle-66-059
# Description: Disposition completeness — 22/22 items have a recorded fate.
# Evidence: .evolve/runs/cycle-66/disposition-memo.md contains a 22-row
#           table with disposition class in each row.
# Author: builder (cycle 66)
# Created: 2026-05-17T00:00:00Z
# Acceptance-of: intent.acceptance_checks[0] "explicit disposition recorded"
#
# metadata:
#   id: 059-disposition-memo-complete
#   cycle: 66
#   task: c66-inbox-disposition-memo
#   severity: HIGH

set -uo pipefail

if [ -n "${EVOLVE_PROJECT_ROOT:-}" ]; then
    REPO_ROOT="$EVOLVE_PROJECT_ROOT"
else
    REPO_ROOT=$(git rev-parse --show-toplevel 2>/dev/null || pwd)
fi
if [ -f "$REPO_ROOT/.git" ]; then
    REPO_ROOT="$(cd "$REPO_ROOT" && cd "$(git rev-parse --git-common-dir)/.." && pwd)"
fi

MEMO="$REPO_ROOT/.evolve/runs/cycle-66/disposition-memo.md"
rc=0

# AC1 — memo file exists
if [ ! -f "$MEMO" ]; then
    echo "RED AC1: $MEMO missing"
    exit 1
fi
echo "GREEN AC1: disposition memo present"

# AC2 — challenge token on line 1 (anti-forgery)
first_line=$(head -1 "$MEMO" 2>/dev/null)
if ! [[ "$first_line" == *"challenge-token:"* ]]; then
    echo "RED AC2: line 1 missing challenge-token marker"
    rc=1
else
    echo "GREEN AC2: challenge token present"
fi

# AC3 — exactly 22 disposition-table rows for inbox items
#   Disposition classes appear in column 5 of the per-item table.
#   Count rows containing one of the four valid classes.
classes_found=$(grep -cE '\*\*(resolved-shipped|partial-resolved|deferred|dropped)\*\*' "$MEMO" || true)
if [ "$classes_found" -lt 22 ]; then
    echo "RED AC3: expected ≥22 disposition rows, found $classes_found"
    rc=1
else
    echo "GREEN AC3: $classes_found disposition rows (≥22 required)"
fi

# AC4 — every inbox item id is referenced in the memo
INBOX_IDS=(
    "user-1778726033-07bb3d9f"
    "user-1778644766-fcf0e86d"
    "c37-inbox-lifecycle-foolproof"
    "c41-eval-score-caps"
    "c42-tool-result-sanitization"
    "c40-ghosh-research-dossier"
    "user-1778645962-3dea8ccb"
    "user-1778653092-c657e1b9"
    "user-1778726033-deae1815"
    "user-1778736450-904b9511"
    "research-cache-per-task"
    "user-1778645968-365e2556"
    "user-1778645973-b5ea5985"
    "user-1778645977-327e78f6"
    "user-1778645981-d653c733"
    "c34-watchdog-failure-class"
    "c35-watchdog-warm-restart"
    "c38-inbox-audit-and-collision"
    "c43-best-attempt-tracking"
    "c36-watchdog-docs"
    "c39-inbox-docs"
    "c44-append-only-discipline"
)
missing=0
for id in "${INBOX_IDS[@]}"; do
    if ! grep -qF "$id" "$MEMO"; then
        echo "RED AC4: inbox id '$id' not referenced in memo"
        missing=$((missing+1))
    fi
done
if [ "$missing" -gt 0 ]; then
    echo "RED AC4: $missing inbox ids missing from memo"
    rc=1
else
    echo "GREEN AC4: all 22 inbox ids referenced"
fi

# AC5 — cycle-31 forensic section present (covers user-1778644766-fcf0e86d)
if ! grep -qE '^## Cycle-31 ship-integrity breach' "$MEMO"; then
    echo "RED AC5: Cycle-31 forensic section missing"
    rc=1
else
    echo "GREEN AC5: Cycle-31 forensic section present"
fi

exit $rc
