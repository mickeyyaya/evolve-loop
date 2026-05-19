#!/usr/bin/env bash
# AC-ID: cycle-89-001-persona-kb-first-pointer
# Description: Verifies all 6 non-Scout phase persona files carry the KB-first
#   research directive (or a one-line pointer to the shared canonical source
#   in docs/architecture/research-tool.md).
# Evidence: scout-report.md:T3 — six persona files listed; intent.md:acceptance_checks
#   first bullet ("grep `kb-search.sh first` returns >=6 files").
# Author: tdd-engineer (cycle-89 Phase C)
# Created: 2026-05-19
# Acceptance-of: build-report.md AC-row "6 persona files contain KB-first directive"
#
# Behavioral: counts the set of persona files (not raw grep hit count) so a
# mutant that pastes the directive twice into one file does NOT compensate for
# a missing file. Each of the six named persona files must independently
# contain the marker string "kb-search.sh first" OR an explicit reference link
# to "research-tool.md#kb-first" — either form satisfies the shared-canonical
# resolution from scout-report.md Premise Resolution #1.
set -uo pipefail

REPO_ROOT="${WORKTREE_PATH:-${EVOLVE_PROJECT_ROOT:-$(git rev-parse --show-toplevel 2>/dev/null || pwd)}}"

PERSONAS="evolve-intent.md evolve-triage.md evolve-tdd-engineer.md evolve-builder.md evolve-auditor.md evolve-retrospective.md"

missing=""
present_count=0
for f in $PERSONAS; do
  path="$REPO_ROOT/agents/$f"
  if [ ! -f "$path" ]; then
    missing="${missing} ${f}(absent)"
    continue
  fi
  # Two accepted forms: literal canonical phrase OR pointer to the ADR anchor.
  hit_count=$(awk '/kb-search\.sh first/ || /research-tool\.md#kb-first/ {n++} END{print n+0}' "$path")
  if [ "${hit_count:-0}" -ge 1 ]; then
    present_count=$((present_count + 1))
  else
    missing="${missing} ${f}(no-directive)"
  fi
done

if [ "$present_count" -ne 6 ]; then
  echo "RED cycle-89-001-persona-kb-first-pointer: only $present_count/6 persona files carry the KB-first directive (missing:${missing})" >&2
  exit 1
fi

echo "GREEN cycle-89-001-persona-kb-first-pointer: all 6 non-Scout persona files carry KB-first directive ($present_count/6)"
exit 0
