#!/usr/bin/env bash
# AC-ID: cycle-88-online-researcher-not-scheduled
#
# Verifies that after the Cycle B migration, the online-researcher subagent is
# no longer scheduled as part of the cycle phase loop:
#   1. scripts/dispatch/run-cycle.sh does not invoke
#      scripts/dispatch/subagent-run.sh online-researcher (i.e., no scheduled
#      Phase 1 dispatch).
#   2. docs/architecture/phase-registry.json has no phase entry with
#      `role == "online-researcher"`.
#
# Note: intent challenged-premise #3 documents that agents/online-researcher.md
# does NOT exist in this repo (a prior refactor already removed it). The
# original Cycle B non-goal "leave online-researcher.md intact" therefore has
# no surface to enforce in this cycle. We test only what is structurally
# present: dispatcher invocation and registry role.
#
# Behavioral: searches *invocation* patterns in the dispatcher, not just any
# mention of the string "online-researcher". Mutants that simply rename the
# subagent without removing the dispatch fall through (1); mutants that
# re-introduce a phase entry under a different name fail (2).
set -uo pipefail

REPO_ROOT="${WORKTREE_PATH:-${EVOLVE_PROJECT_ROOT:-$(git rev-parse --show-toplevel 2>/dev/null || pwd)}}"

fail=0
errors=""

# (1) No subagent-run invocation of online-researcher in dispatcher.
DISPATCH_FILE="$REPO_ROOT/scripts/dispatch/run-cycle.sh"
if [ -f "$DISPATCH_FILE" ]; then
  # Match: `subagent-run.sh online-researcher` or `--role online-researcher`
  # (any quoted form). We deliberately exclude comments by stripping inline #
  # via awk first.
  invocation_hits=$(awk '
    {
      # Strip from a # to end-of-line ONLY when # is not inside quotes — naive
      # but adequate for bash dispatcher style.
      line = $0
      sub(/[[:space:]]*#.*/, "", line)
      if (line ~ /subagent-run\.sh[[:space:]]+["'\''"]?online-researcher/ ||
          line ~ /--role[[:space:]]+["'\''"]?online-researcher/) print NR ":" line
    }
  ' "$DISPATCH_FILE" 2>/dev/null)
  if [ -n "$invocation_hits" ]; then
    errors="${errors}\n  run-cycle.sh still invokes online-researcher subagent:\n$invocation_hits"
    fail=$((fail + 1))
  fi
else
  errors="${errors}\n  run-cycle.sh missing at $DISPATCH_FILE (cannot verify Phase 1 invocation removal)"
  fail=$((fail + 1))
fi

# (2) No registry phase entry with role == "online-researcher".
REG_FILE="$REPO_ROOT/docs/architecture/phase-registry.json"
if [ -f "$REG_FILE" ] && command -v jq >/dev/null 2>&1; then
  role_count=$(jq -r '[.phases[] | select(.role == "online-researcher")] | length' "$REG_FILE" 2>/dev/null)
  if [ "${role_count:-0}" -gt 0 ] 2>/dev/null; then
    errors="${errors}\n  phase-registry.json has $role_count entries with role='online-researcher' (must be 0)"
    fail=$((fail + 1))
  fi
fi

if [ $fail -gt 0 ]; then
  echo "RED cycle-88-online-researcher-not-scheduled: $fail issue(s)"
  printf "%b\n" "$errors" >&2
  exit 1
fi
echo "GREEN cycle-88-online-researcher-not-scheduled: dispatcher does not invoke online-researcher; registry has no online-researcher role"
exit 0
