#!/usr/bin/env bash
# AC-ID:         cycle-44-004
# Description:   knowledge-base/research/token-reduction-2026-may.md updated with cycle-44 sources
# Evidence:      knowledge-base/research/token-reduction-2026-may.md
# Author:        builder
# Created:       2026-05-14T00:00:00Z
# Acceptance-of: build-report.md T2 (KB update with cycle-44 research)
set -uo pipefail

REPO_ROOT="$(git rev-parse --show-toplevel 2>/dev/null || echo ".")"
FILE="$REPO_ROOT/knowledge-base/research/token-reduction-2026-may.md"

[ -f "$FILE" ] || { echo "FAIL: token-reduction-2026-may.md not found"; exit 1; }

# Must contain Source 11 (arXiv:2604.19572 — Observational Context Compression)
grep -q "2604.19572\|Observational Context Compression" "$FILE" || { echo "FAIL: Source 11 (arXiv:2604.19572) not found in KB"; exit 1; }

# Must contain Source 12 (arXiv:2412.18547 — Token-Budget-Aware)
grep -q "2412.18547\|Token-Budget-Aware" "$FILE" || { echo "FAIL: Source 12 (arXiv:2412.18547) not found in KB"; exit 1; }

# Must contain Anthropic compaction reference
grep -q "compact-2026-01-12\|Compaction API" "$FILE" || { echo "FAIL: Anthropic Compaction API (Source 13) not found in KB"; exit 1; }

# Must reference cycle-44 research summary
grep -q "Cycle-44\|cycle 44\|cycle-44" "$FILE" || { echo "FAIL: cycle-44 research summary not found in KB"; exit 1; }

echo "PASS: knowledge-base/research/token-reduction-2026-may.md updated with cycle-44 sources (11-14)"
exit 0
