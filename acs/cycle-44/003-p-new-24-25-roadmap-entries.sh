#!/usr/bin/env bash
# AC-ID:         cycle-44-003
# Description:   token-reduction-roadmap.md contains P-NEW-24 and P-NEW-25 sections
# Evidence:      docs/architecture/token-reduction-roadmap.md
# Author:        builder
# Created:       2026-05-14T00:00:00Z
# Acceptance-of: build-report.md T2 (P-NEW-24/25 roadmap entries)
set -uo pipefail

REPO_ROOT="$(git rev-parse --show-toplevel 2>/dev/null || echo ".")"
FILE="$REPO_ROOT/docs/architecture/token-reduction-roadmap.md"

[ -f "$FILE" ] || { echo "FAIL: token-reduction-roadmap.md not found"; exit 1; }

grep -q "P-NEW-24" "$FILE" || { echo "FAIL: P-NEW-24 not found in roadmap"; exit 1; }
grep -q "P-NEW-25" "$FILE" || { echo "FAIL: P-NEW-25 not found in roadmap"; exit 1; }

# Both must appear as section headers (## P-NEW-24 and ## P-NEW-25)
grep -q "## P-NEW-24" "$FILE" || { echo "FAIL: ## P-NEW-24 section header not found"; exit 1; }
grep -q "## P-NEW-25" "$FILE" || { echo "FAIL: ## P-NEW-25 section header not found"; exit 1; }

# P-NEW-23 must be marked DONE
grep -q "P-NEW-23.*DONE\|DONE.*P-NEW-23" "$FILE" || { echo "FAIL: P-NEW-23 not marked DONE in roadmap"; exit 1; }

echo "PASS: roadmap has P-NEW-24, P-NEW-25 sections and P-NEW-23 marked DONE"
exit 0
