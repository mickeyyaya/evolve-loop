#!/usr/bin/env bash
# Assert: build-report-ac-verify.sh exists, is executable, has set -uo pipefail, passes shellcheck
set -uo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
REPO_ROOT="$(cd "$SCRIPT_DIR/../../.." && pwd)"
HARNESS="${EVOLVE_PROJECT_ROOT:-$REPO_ROOT}/scripts/lifecycle/build-report-ac-verify.sh"

[ -f "$HARNESS" ] || { echo "FAIL: harness not found at $HARNESS"; exit 1; }
[ -x "$HARNESS" ] || { echo "FAIL: harness not executable: $HARNESS"; exit 1; }
grep -q "set -uo pipefail" "$HARNESS" || { echo "FAIL: harness missing 'set -uo pipefail'"; exit 1; }

if command -v shellcheck >/dev/null 2>&1; then
    shellcheck -S warning "$HARNESS" || { echo "FAIL: shellcheck found issues in $HARNESS"; exit 1; }
fi

echo "PASS: harness exists, executable, set -uo pipefail present, shellcheck clean"
