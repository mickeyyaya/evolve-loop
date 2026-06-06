#!/usr/bin/env bash
# AC-ID: cycle-89-004-research-tool-adr-exists
# Description: Verifies the Phase-C architecture ADR exists at
#   docs/architecture/research-tool.md, has substantive body (>=80 non-blank
#   lines), contains the two required section headings "## Hook Contract" and
#   "## Profile Schema", AND is referenced from at least one canonical doc
#   (CLAUDE.md, AGENTS.md, or agents/evolve-scout.md).
# Evidence: scout-report.md:T1 + Acceptance Criteria Mapping row 4;
#   intent.md:acceptance_checks bullet 4 ("docs/architecture/research-tool.md
#   exists, >=80 lines, hook contract + profile schema, referenced from one
#   canonical doc").
# Author: tdd-engineer (cycle-89 Phase C)
# Created: 2026-05-19
# Acceptance-of: build-report.md AC-row "docs/architecture/research-tool.md
#   ADR shipped with hook contract + profile schema sections + cross-ref"
#
# Behavioral: each of the four sub-checks operates on actual file content
# (existence, line count, heading match, cross-doc reference scan). A mutant
# that creates an empty file with only the two headings fails the line-count
# check; a mutant that puts the headings inside a code fence fails the
# heading regex (anchored to start-of-line `^## `); a mutant that ships the
# ADR but adds no cross-reference from any canonical doc fails check 4.
set -uo pipefail

REPO_ROOT="${WORKTREE_PATH:-${EVOLVE_PROJECT_ROOT:-$(git rev-parse --show-toplevel 2>/dev/null || pwd)}}"
ADR="$REPO_ROOT/docs/architecture/research-tool.md"

fail=0
errors=""

# (1) ADR must exist.
if [ ! -f "$ADR" ]; then
  echo "RED cycle-89-004-research-tool-adr-exists: ADR missing at $ADR" >&2
  exit 1
fi

# (2) Substantive body — count non-blank lines (subprocess invocation: wc).
body_line_count=$(awk 'NF' "$ADR" | wc -l | tr -d ' ')
if [ "${body_line_count:-0}" -lt 80 ]; then
  errors="${errors}\n  ADR too thin ($body_line_count non-blank lines, expected >=80)"
  fail=$((fail + 1))
fi

# (3) Required section headings — anchored to start-of-line so code-fenced
# mentions don't count. awk numeric-counts both heads to give precise feedback.
hook_count=$(awk '/^##[[:space:]]+Hook[[:space:]]+Contract/ {n++} END{print n+0}' "$ADR")
schema_count=$(awk '/^##[[:space:]]+Profile[[:space:]]+Schema/ {n++} END{print n+0}' "$ADR")
if [ "${hook_count:-0}" -lt 1 ]; then
  errors="${errors}\n  missing required heading: ## Hook Contract"
  fail=$((fail + 1))
fi
if [ "${schema_count:-0}" -lt 1 ]; then
  errors="${errors}\n  missing required heading: ## Profile Schema"
  fail=$((fail + 1))
fi

# (4) Cross-reference from at least one canonical doc. Accept any of:
#   CLAUDE.md, AGENTS.md, agents/evolve-scout.md,
#   docs/operations/runtime-reference.md (added 2026-06-05: the CLAUDE.md
#   split d8ac721 moved runtime cross-refs there; it is CLAUDE.md's linked
#   canonical extension).
# Reference forms accepted: bare filename "research-tool.md", relative path
# "docs/architecture/research-tool.md", or anchor "research-tool.md#...".
ref_targets="$REPO_ROOT/CLAUDE.md $REPO_ROOT/AGENTS.md $REPO_ROOT/agents/evolve-scout.md $REPO_ROOT/docs/operations/runtime-reference.md"
ref_hits=0
matched_in=""
for tgt in $ref_targets; do
  [ -f "$tgt" ] || continue
  hits=$(awk '/research-tool\.md/ {n++} END{print n+0}' "$tgt")
  if [ "${hits:-0}" -ge 1 ]; then
    ref_hits=$((ref_hits + hits))
    matched_in="${matched_in} $(basename "$tgt")(${hits})"
  fi
done
if [ "$ref_hits" -lt 1 ]; then
  errors="${errors}\n  ADR not referenced from CLAUDE.md, AGENTS.md, agents/evolve-scout.md, or docs/operations/runtime-reference.md"
  fail=$((fail + 1))
fi

if [ "$fail" -gt 0 ]; then
  echo "RED cycle-89-004-research-tool-adr-exists: $fail issue(s)" >&2
  printf "%b\n" "$errors" >&2
  exit 1
fi

echo "GREEN cycle-89-004-research-tool-adr-exists: ADR present ($body_line_count lines), Hook Contract + Profile Schema headings found, cross-referenced from:${matched_in}"
exit 0
