#!/usr/bin/env bash
# AC-ID: cycle-89-003-claude-md-research-env-vars
# Description: Verifies CLAUDE.md current-behavior env-var table carries the
#   research-tool rows for the 3 confirmed env vars (EVOLVE_ALLOW_DEEP_RESEARCH,
#   EVOLVE_RESEARCH_HOOK_DISABLED, EVOLVE_KB_SEARCH_PATHS). The 4th name
#   EVOLVE_RESEARCH_QUOTA_SOFT is checked SOFT-only because scout-report.md
#   Finding F3 flagged it as not-yet-implemented in scripts; Builder may
#   legitimately omit it or document as "planned" — both forms pass.
# Evidence: scout-report.md:T4 + Finding F3; intent.md:acceptance_checks
#   bullet 3 ("CLAUDE.md env-var table contains rows for all 4 named vars").
# Author: tdd-engineer (cycle-89 Phase C)
# Created: 2026-05-19
# Acceptance-of: build-report.md AC-row "CLAUDE.md env-var table has 4 new
#   research-tool rows"
#
# Behavioral: parses CLAUDE.md as a markdown table, identifies env-var-name
# rows containing the target tokens, and verifies each row has non-empty
# Default + Effect columns. A mutant that merely mentions the env var name in
# prose (outside a table row) does NOT satisfy the check because the predicate
# requires `|` separator structure.
set -uo pipefail

REPO_ROOT="${WORKTREE_PATH:-${EVOLVE_PROJECT_ROOT:-$(git rev-parse --show-toplevel 2>/dev/null || pwd)}}"
CLAUDE_MD="$REPO_ROOT/CLAUDE.md"

if [ ! -f "$CLAUDE_MD" ]; then
  echo "RED cycle-89-003-claude-md-research-env-vars: CLAUDE.md not found at $CLAUDE_MD" >&2
  exit 1
fi

REQUIRED="EVOLVE_ALLOW_DEEP_RESEARCH EVOLVE_RESEARCH_HOOK_DISABLED EVOLVE_KB_SEARCH_PATHS"

missing=""
malformed=""
for var in $REQUIRED; do
  # Row form: `| <subsystem> | \`<VAR>\` | <default> | <effect> |`
  # Use awk to find lines containing the var name inside a backtick within
  # a pipe-delimited table row; then require >=4 non-empty cells.
  row_check=$(awk -v var="$var" '
    /^\|/ && index($0, var) {
      # Split on pipe; trim each field; require >=4 non-empty fields after
      # the leading empty (the leading `|` produces an empty field at index 1).
      n = split($0, cells, "|")
      filled = 0
      for (i = 1; i <= n; i++) {
        gsub(/^[[:space:]]+|[[:space:]]+$/, "", cells[i])
        if (length(cells[i]) > 0) filled++
      }
      if (filled >= 4) { found = 1; exit }
    }
    END { print (found ? "OK" : "MALFORMED") }
  ' "$CLAUDE_MD")

  if [ "$row_check" = "OK" ]; then
    : # row present and well-formed
  elif grep -qF "$var" "$CLAUDE_MD"; then
    # Var name appears but not in well-formed row → malformed.
    malformed="${malformed} ${var}"
  else
    # Var name absent entirely.
    missing="${missing} ${var}"
  fi
done

fail=0
if [ -n "$missing" ]; then
  echo "RED cycle-89-003-claude-md-research-env-vars: missing env-var rows in CLAUDE.md:${missing}" >&2
  fail=1
fi
if [ -n "$malformed" ]; then
  echo "RED cycle-89-003-claude-md-research-env-vars: env-var names present but not in valid table row:${malformed}" >&2
  fail=1
fi

# Soft-check (informational): EVOLVE_RESEARCH_QUOTA_SOFT. Logged but does not
# fail the predicate (scout flagged as TBD; Builder may omit).
if ! grep -qF "EVOLVE_RESEARCH_QUOTA_SOFT" "$CLAUDE_MD"; then
  echo "[info] EVOLVE_RESEARCH_QUOTA_SOFT not present in CLAUDE.md (acceptable — scout flagged unimplemented in scripts)" >&2
fi

if [ "$fail" -ne 0 ]; then
  exit 1
fi

echo "GREEN cycle-89-003-claude-md-research-env-vars: 3 required env-var rows present and well-formed"
exit 0
