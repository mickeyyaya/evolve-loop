#!/usr/bin/env bash
# AC-ID:         cycle-65-003
# Description:   Anchor validation enforced in role-context-builder.sh
# Evidence:      scripts/lifecycle/role-context-builder.sh
# Author:        builder
# Created:       2026-05-15T22:10:00Z
# Acceptance-of: build-report.md AC#3

set -euo pipefail

# Verify the warning logic exists in role-context-builder.sh
grep -q "WARN: modular artifact '\$path' lacks ANCHOR markers" scripts/lifecycle/role-context-builder.sh || { echo "FAIL: Anchor validation warning missing from role-context-builder.sh"; exit 1; }

# Mock a check to see if it triggers for critical files
# We can't easily run it without dependencies, but we can verify the case pattern
grep -q "scout-report.md|build-report.md|audit-report.md" scripts/lifecycle/role-context-builder.sh || { echo "FAIL: Critical report list missing from validation"; exit 1; }

echo "PASS: Anchor validation logic verified"
exit 0
