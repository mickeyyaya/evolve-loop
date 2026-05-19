#!/usr/bin/env bash
# ACS predicate — cycle 85
# Verifies that promote-acs-to-regression.sh has the workspace fallback path logic.
set -uo pipefail

SCRIPT="${EVOLVE_PROJECT_ROOT:-$(git rev-parse --show-toplevel 2>/dev/null || pwd)}/scripts/utility/promote-acs-to-regression.sh"

[ -f "$SCRIPT" ] || { echo "FAIL: promote-acs-to-regression.sh not found at $SCRIPT" >&2; exit 1; }

if ! grep -qF 'WORKSPACE_SRC' "$SCRIPT"; then
    echo "FAIL: promote-acs-to-regression.sh missing WORKSPACE_SRC fallback variable" >&2
    exit 1
fi

if ! grep -qF '.evolve/runs/cycle-' "$SCRIPT"; then
    echo "FAIL: promote-acs-to-regression.sh missing workspace-relative fallback path check" >&2
    exit 1
fi

echo "PASS: promote-acs-to-regression.sh has workspace fallback path logic"
exit 0
