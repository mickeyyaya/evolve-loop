#!/usr/bin/env bash
# AC-ID:         cycle-227-001
# Description:   AgentForPhase tries .evolve/phases/<name>/agent.md first, falls back to agents/<name>.md; Agent() delegates to AgentForPhase
# Evidence:      go/internal/prompts/prompts.go:75-81 + go/internal/prompts/agentforphase_test.go
# Author:        tester
# Created:       2026-06-06T00:00:00Z
# Acceptance-of: build-report.md AC#1 — AgentForPhase two-path persona resolution (Mode 1 fix)

set -uo pipefail

WORKTREE="${EVOLVE_WORKTREE_PATH:-${WORKTREE_PATH:-$(git rev-parse --show-toplevel 2>/dev/null || pwd)}}"

# Structural: AgentForPhase must be defined and Agent() must delegate to it.
PROMPTS="$WORKTREE/go/internal/prompts/prompts.go"
[ -f "$PROMPTS" ] || { echo "RED: $PROMPTS not found" >&2; exit 1; }

grep -q "func (l \*Loader) AgentForPhase(" "$PROMPTS" \
  || { echo "RED: AgentForPhase not defined in $PROMPTS" >&2; exit 1; }

grep -q "return l.AgentForPhase(name)" "$PROMPTS" \
  || { echo "RED: Agent() does not delegate to AgentForPhase in $PROMPTS" >&2; exit 1; }

# Behavioral: run the full AgentForPhase test suite (3 contract tests + prior tests).
# Phase-dir wins, agents-dir fallback, and ErrNotExist-wrapped miss are all tested.
cd "$WORKTREE/go" || { echo "RED: cannot cd to $WORKTREE/go" >&2; exit 1; }

if ! go test ./internal/prompts/... -run "TestAgentForPhase" -timeout 60s 2>&1; then
  echo "RED: AgentForPhase test suite FAILED" >&2
  exit 1
fi

echo "GREEN: AgentForPhase two-path lookup and Agent() delegation verified" >&2
exit 0
