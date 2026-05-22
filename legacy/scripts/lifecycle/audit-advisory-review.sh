#!/usr/bin/env bash
# audit-advisory-review.sh — advisory code-review pass using code-review-simplify SKILL
#
# Invoked by phase-gate.sh gate_audit_to_ship AFTER the audit verdict is bound.
# Writes an advisory artifact to $WORKSPACE/audit-advisory-review.md.
# Returns exit 0 always — advisory; never blocks the cycle.
# Gated by EVOLVE_AUDIT_ADVISORY_REVIEW=1 (default 0, opt-in only).
#
# Bash 3.2 portable (no declare -A, mapfile, ${var^^}, GNU sed/date).

set -uo pipefail

CYCLE="${1:?Usage: audit-advisory-review.sh <cycle> <workspace>}"
WORKSPACE="${2:?Missing workspace path}"

OUT="$WORKSPACE/audit-advisory-review.md"
TMPOUT="${OUT}.tmp.$$"

# No-op unless opt-in flag is explicitly set
if [ "${EVOLVE_AUDIT_ADVISORY_REVIEW:-0}" != "1" ]; then
    exit 0
fi

# Capture diff stat (git diff against previous commit; fallback to empty)
DIFF_STAT=""
if git diff HEAD~1 --stat 2>/dev/null | grep -q .; then
    DIFF_STAT=$(git diff HEAD~1 --stat 2>/dev/null)
elif git diff --cached --stat 2>/dev/null | grep -q .; then
    DIFF_STAT=$(git diff --cached --stat 2>/dev/null)
else
    DIFF_STAT="(no diff available — fresh worktree or no prior commits)"
fi

# Capture brief diff (first 100 lines) for the advisory reader
DIFF_BRIEF=""
if git diff HEAD~1 2>/dev/null | head -100 | grep -q .; then
    DIFF_BRIEF=$(git diff HEAD~1 2>/dev/null | head -100)
else
    DIFF_BRIEF="(diff unavailable)"
fi

# Write advisory artifact atomically
printf '%s\n' \
    "<!-- advisory-review: opt-in, read-only, non-verdict-bearing -->" \
    "# Audit Advisory Review — Cycle ${CYCLE}" \
    "" \
    "> **Note:** This artifact was produced by \`audit-advisory-review.sh\`, an" \
    "> opt-in advisory hook (\`EVOLVE_AUDIT_ADVISORY_REVIEW=1\`). It does NOT affect" \
    "> the audit verdict or the ship-gate decision. Operators use this for" \
    "> observability and continuous improvement." \
    "" \
    "## Status" \
    "" \
    "- Advisory hook: **active** (\`EVOLVE_AUDIT_ADVISORY_REVIEW=1\`)" \
    "- Verdict authority: **none** (advisory only; audit verdict already bound)" \
    "- Artifact binding: **excluded** (this file is NOT part of audit-report SHA)" \
    "" \
    "## Diff Summary" \
    "" \
    "\`\`\`" \
    "$DIFF_STAT" \
    "\`\`\`" \
    "" \
    "## Code Review + Simplify — Guidance (skills/code-review-simplify/SKILL.md)" \
    "" \
    "The diff above is the input surface for advisory code review. Operators or" \
    "future Scouts may invoke \`/code-review-simplify\` against the diff manually." \
    "" \
    "Dimensional scoring criteria (from code-review-simplify SKILL):" \
    "" \
    "| Dimension | Weight | Advisory threshold |" \
    "|-----------|--------|--------------------|" \
    "| Correctness | 0.35 | < 0.8 → flag for next cycle |" \
    "| Security | 0.30 | < 0.9 → flag IMMEDIATELY |" \
    "| Performance | 0.20 | < 0.7 → flag for next cycle |" \
    "| Maintainability | 0.15 | < 0.7 → simplification suggestion |" \
    "" \
    "## Diff (first 100 lines)" \
    "" \
    "\`\`\`diff" \
    "$DIFF_BRIEF" \
    "\`\`\`" \
    > "$TMPOUT"

mv -f "$TMPOUT" "$OUT"
exit 0
