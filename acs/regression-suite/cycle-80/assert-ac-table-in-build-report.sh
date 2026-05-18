#!/usr/bin/env bash
# Assert: .evolve/runs/cycle-80/build-report.md contains AC-TABLE-BEGIN/END anchors with harness-stamp
set -uo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
REPO_ROOT="$(cd "$SCRIPT_DIR/../../.." && pwd)"
BUILD_REPORT="${EVOLVE_PROJECT_ROOT:-$REPO_ROOT}/.evolve/runs/cycle-80/build-report.md"

[ -f "$BUILD_REPORT" ] || { echo "FAIL: build-report.md not found at $BUILD_REPORT"; exit 1; }
grep -q "<!-- AC-TABLE-BEGIN -->" "$BUILD_REPORT" || { echo "FAIL: AC-TABLE-BEGIN anchor missing"; exit 1; }
grep -q "<!-- AC-TABLE-END -->" "$BUILD_REPORT" || { echo "FAIL: AC-TABLE-END anchor missing"; exit 1; }
grep -q "<!-- harness-stamp: build-report-ac-verify.sh" "$BUILD_REPORT" || { echo "FAIL: harness-stamp line missing"; exit 1; }

echo "PASS: AC-TABLE-BEGIN/END anchors and harness-stamp present in build-report.md"
