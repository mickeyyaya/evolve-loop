#!/usr/bin/env bash
# AC-ID:         cycle-230-001
# Description:   agents/evolve-auditor.md trimmed to <=300 lines (cycle-77 regression predicate GREEN) with relocated sections preserved in the reference doc (moved, not deleted)
# Evidence:      acs/regression-suite/cycle-77/001-auditor-stage8-cold-move.sh + agents/evolve-auditor-reference.md
# Author:        tdd-engineer
# Created:       2026-06-06T00:00:00Z
# Acceptance-of: scout-report.md Task 1 (auditor-doc-trim) — unblocks the ship gate broken since commit 48f8ff7 (319 > 300 lines)

set -uo pipefail

WORKTREE="${EVOLVE_WORKTREE_PATH:-${WORKTREE_PATH:-$(git rev-parse --show-toplevel 2>/dev/null || pwd)}}"
cd "$WORKTREE" || { echo "RED: cannot cd to $WORKTREE" >&2; exit 1; }

REGRESSION="acs/regression-suite/cycle-77/001-auditor-stage8-cold-move.sh"
REFERENCE="agents/evolve-auditor-reference.md"
AUDITOR="agents/evolve-auditor.md"

# Behavioral: run the actual cycle-77 regression predicate as a subprocess.
# This is the exact gate that has FAILed every ship since cycle 228 — the
# task is done only when this subprocess exits 0.
if ! bash "$REGRESSION"; then
    echo "RED: cycle-77 regression predicate still failing ($(wc -l < "$AUDITOR" | tr -d ' ') lines in $AUDITOR)" >&2
    exit 1
fi

# Content preservation (negative axis — "trim" must be a MOVE, not a delete):
# each relocated rule must still exist somewhere reachable. The three late
# sections named in scout-report Task 1 must survive in persona OR reference.
for token in "Reflection-sycophancy defect check" "WARN-elevation hardening" "Hypothesis falsification emission"; do
    if ! grep -qF "$token" "$REFERENCE" && ! grep -qF "$token" "$AUDITOR"; then
        echo "RED: section \"$token\" found in neither $AUDITOR nor $REFERENCE — content was deleted, not moved" >&2
        exit 1
    fi
done

# Each relocated section must remain discoverable FROM the persona: a pointer
# to evolve-auditor-reference.md must exist in the trimmed auditor doc.
if ! grep -q 'evolve-auditor-reference\.md' "$AUDITOR"; then
    echo "RED: no pointer to evolve-auditor-reference.md in $AUDITOR" >&2
    exit 1
fi

echo "GREEN: cycle-77 regression predicate exits 0; relocated sections preserved; persona points to reference" >&2
exit 0
