#!/usr/bin/env bash
# acs-predicate: config-check — inherently a doc-content check: the AC is a
#                text edit to agents/evolve-auditor.md (instruction → note).
#                The kernel-resolution BEHAVIOR is covered by predicates
#                002/003 (resolveACSSuiteRoot tests).
# AC-ID:         cycle-232-t2-AC5
# Description:   auditor C0 block demoted — documents kernel-owned --root resolution; imperative snippet removed
# Evidence:      agents/evolve-auditor.md
# Author:        tdd-engineer
# Created:       2026-06-06T00:00:00Z
# Acceptance-of: scout-report.md Task 2 (topology-handles-and-ship-preflight) AC5

set -uo pipefail

ROOT="${EVOLVE_WORKTREE_PATH:-${WORKTREE_PATH:-$(git rev-parse --show-toplevel 2>/dev/null || pwd)}}"
DOC="$ROOT/agents/evolve-auditor.md"

[ -f "$DOC" ] || { echo "RED: $DOC missing" >&2; exit 1; }

# Positive: the block must document kernel-owned resolution from
# cycle-state.json (scout AC5 check).
grep -qE 'kernel.owned|cycle-state\.json' "$DOC" \
  || { echo "RED: auditor doc does not mention kernel-owned --root resolution / cycle-state.json" >&2; exit 1; }

# Negative (anti-no-op): the old IMPERATIVE instruction must be gone — the
# C0 block told the auditor LLM to run the root-resolution snippet itself
# ("Run EXACTLY (no improvised roots)"). After demotion it is a
# defense-in-depth note, not an instruction.
if grep -qF 'Run EXACTLY (no improvised roots)' "$DOC"; then
  echo "RED: imperative C0 instruction still present — block not demoted to documentation" >&2
  exit 1
fi

echo "GREEN: auditor C0 block demoted to kernel-owned documentation note" >&2
exit 0
