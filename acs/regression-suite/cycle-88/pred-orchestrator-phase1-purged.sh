#!/usr/bin/env bash
# AC-ID: cycle-88-orchestrator-phase1-purged
#
# Verifies Cycle B persona edit on agents/evolve-orchestrator.md:
#   1. The Phase Outcomes table no longer lists a "research" row pointing to
#      scout (legacy phase-loop documentation).
#   2. "Phase 1" is not referenced as a current phase anywhere in the
#      orchestrator persona.
#   3. The Phase Outcomes table DOES list a "discover" row (the new flow
#      documentation; mutants that simply delete the line without replacing it
#      lose the doc and fail this criterion).
#
# Behavioral: looks at the actual table cell, not just any "research" mention.
# A mutant that drops the row entirely fails (3); a mutant that leaves the row
# unchanged fails (1); a mutant that renames "research" to "discover" in the
# row but leaves "Phase 1" elsewhere fails (2).
set -uo pipefail

REPO_ROOT="${WORKTREE_PATH:-${EVOLVE_PROJECT_ROOT:-$(git rev-parse --show-toplevel 2>/dev/null || pwd)}}"
ORCH_FILE="$REPO_ROOT/agents/evolve-orchestrator.md"

if [ ! -f "$ORCH_FILE" ]; then
  echo "RED cycle-88-orchestrator-phase1-purged: agents/evolve-orchestrator.md missing at $ORCH_FILE"
  exit 1
fi

fail=0
errors=""

# (1) No "research | scout" table row.
#     Markdown table cells: `| research | scout |` (any whitespace).
if grep -qE '^\|[[:space:]]*research[[:space:]]*\|[[:space:]]*scout[[:space:]]*\|' "$ORCH_FILE"; then
  errors="${errors}\n  evolve-orchestrator.md still has Phase Outcomes table row '| research | scout |' (must be 'discover | scout')"
  fail=$((fail + 1))
fi

# (2) No "Phase 1" mention as a current phase. Allow it inside historical /
#     migration notes (lines containing "retired" or "cycle-88" or "migration").
# Use grep -n + filter:
while IFS= read -r line; do
  if printf '%s' "$line" | grep -qiE 'retired|migrat|cycle-88|legacy|deprecat'; then
    continue
  fi
  errors="${errors}\n  evolve-orchestrator.md references 'Phase 1' without migration context: $line"
  fail=$((fail + 1))
done < <(grep -nE 'Phase[[:space:]]+1' "$ORCH_FILE" 2>/dev/null || true)

# (3) Must have a "discover | scout" table row (or equivalent) so the
#     phase-flow documentation actually documents the new flow. Accept either
#     `| discover | scout |` (replacement) or any line documenting "discover"
#     mapped to scout in a markdown-table context.
if ! grep -qE '^\|[[:space:]]*discover[[:space:]]*\|[[:space:]]*scout[[:space:]]*\|' "$ORCH_FILE"; then
  errors="${errors}\n  evolve-orchestrator.md missing Phase Outcomes row '| discover | scout |' (must replace 'research | scout')"
  fail=$((fail + 1))
fi

if [ $fail -gt 0 ]; then
  echo "RED cycle-88-orchestrator-phase1-purged: $fail issue(s)"
  printf "%b\n" "$errors" >&2
  exit 1
fi
echo "GREEN cycle-88-orchestrator-phase1-purged: Phase Outcomes table row updated to 'discover | scout'; no stray 'Phase 1' references outside migration context"
exit 0
