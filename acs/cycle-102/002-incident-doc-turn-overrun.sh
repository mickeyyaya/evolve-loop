#!/usr/bin/env bash
# AC-ID: cycle-102-002-incident-doc-turn-overrun
# AC-source: cycle-102/scout-report.md AC-5 (carryover abnormal-turn-overrun-c99)
#
# Behavioral predicate: the cycle-99/100 turn-overrun incident doc
# must exist at the canonical path, be tracked by git, and contain
# enough substance to be a real root-cause record (>=30 non-blank
# lines) — not a stub.
#
# Path: docs/operations/incidents/cycle-99-100-turn-overrun.md
#
# Predicate is BEHAVIORAL on side-effect: it invokes `git ls-files
# --error-unmatch` and `grep -cvE` as subprocesses to verify tracking
# and content density (acs/AGENTS.md acceptable-grep waiver: mixed
# grep + subprocess invocation; the grep is on a *side-effect file*,
# not on source-code semantics).
#
# Dual-check pattern: file + git-tracking. Required by cycle-93 ACS
# guide to catch gitignored worktree artifacts.
#
# Content density requirement: >=30 non-blank lines. Lower than this
# indicates a stub or a memo-only conclusion, which is the failure
# mode this cycle is designed to prevent (intent.md: "Builder should
# NOT close a carryover item via a memo-only narrative").
#
# Substance requirement: the doc must reference at least two of the
# four affected agents (triage, intent, scout, builder) by name —
# proving it actually documents the recurrence pattern.
#
# Bash 3.2 compatible.
#
# Exit codes:
#   0 = GREEN (doc exists, tracked, dense, references >=2 agents)
#   1 = RED   (missing, untracked, thin, or doesn't reference agents)
set -uo pipefail

REPO_ROOT="$(git rev-parse --show-toplevel 2>/dev/null)"
if [ -z "$REPO_ROOT" ]; then
  echo "RED: not inside a git work tree" >&2
  exit 1
fi
cd "$REPO_ROOT" || { echo "RED: cd to repo root failed" >&2; exit 1; }

DOC="docs/operations/incidents/cycle-99-100-turn-overrun.md"
MIN_LINES=30
MIN_AGENT_REFS=2

# Disk presence
if [ ! -f "$DOC" ]; then
  echo "RED: $DOC missing on disk" >&2
  exit 1
fi

# Git tracking — catches gitignored worktree files
if ! git ls-files --error-unmatch "$DOC" >/dev/null 2>&1; then
  echo "RED: $DOC exists on disk but is not tracked by git (worktree-only — would be dropped at ship)" >&2
  exit 1
fi

# Content density: non-blank line count
nonblank=$(grep -cvE '^[[:space:]]*$' "$DOC" 2>/dev/null || echo 0)
if [ "$nonblank" -lt "$MIN_LINES" ]; then
  echo "RED: $DOC has only $nonblank non-blank lines (need >=$MIN_LINES — stub/memo not acceptable)" >&2
  exit 1
fi

# Substance: must reference at least 2 of the 4 affected agents.
# Use case-insensitive whole-token match to avoid false positives
# (e.g. "trigger" matching "triage" prefix).
agent_refs=0
for agent in triage intent scout builder; do
  if grep -qiE "(^|[^a-zA-Z])${agent}([^a-zA-Z]|$)" "$DOC" 2>/dev/null; then
    agent_refs=$(( agent_refs + 1 ))
  fi
done

if [ "$agent_refs" -lt "$MIN_AGENT_REFS" ]; then
  echo "RED: $DOC references only $agent_refs of 4 affected agents (need >=$MIN_AGENT_REFS to qualify as recurrence-pattern documentation)" >&2
  exit 1
fi

echo "GREEN: $DOC exists ($nonblank non-blank lines, references $agent_refs/4 affected agents), git-tracked"
exit 0
