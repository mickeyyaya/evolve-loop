#!/usr/bin/env bash
# AC-ID:         cycle-227-006
# Description:   LedgerEntry.Source (omitempty) added; recordRoutingDecision populates it from RouterDecision.SkipSources
# Evidence:      go/internal/core/ports.go:167,194,223 + go/internal/core/skip_source_test.go
# Author:        tester
# Created:       2026-06-06T00:00:00Z
# Acceptance-of: build-report.md AC#6 — skip-source attribution on LedgerEntry

set -uo pipefail

WORKTREE="${EVOLVE_WORKTREE_PATH:-${WORKTREE_PATH:-$(git rev-parse --show-toplevel 2>/dev/null || pwd)}}"

PORTS="$WORKTREE/go/internal/core/ports.go"
[ -f "$PORTS" ] || { echo "RED: $PORTS not found" >&2; exit 1; }

# Structural: Source field with omitempty must exist in LedgerEntry.
grep -q 'Source.*omitempty' "$PORTS" \
  || { echo "RED: LedgerEntry.Source omitempty not found in $PORTS" >&2; exit 1; }

ROUTER="$WORKTREE/go/internal/router/router.go"
grep -q "SkipSources" "$ROUTER" \
  || { echo "RED: RouterDecision.SkipSources not found in $ROUTER" >&2; exit 1; }

# Behavioral: attribution round-trip and omitempty hash-chain compat.
cd "$WORKTREE/go" || { echo "RED: cannot cd to $WORKTREE/go" >&2; exit 1; }

if ! go test ./internal/core/... \
  -run "TestPhaseSkipped_SourceAttribution|TestLedgerEntry_SourceOmittedWhenEmpty" \
  -timeout 60s 2>&1; then
  echo "RED: LedgerEntry.Source attribution test suite FAILED" >&2
  exit 1
fi

echo "GREEN: LedgerEntry.Source (omitempty) wired; recordRoutingDecision populates from SkipSources" >&2
exit 0
