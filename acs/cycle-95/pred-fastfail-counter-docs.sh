#!/usr/bin/env bash
# AC-ID: cycle-95-fastfail-counter-docs
#
# Verifies O-1 carryover: the per-workspace-per-cycle semantics of the
# fast-fail counter are explicitly documented in subagent-run.sh near the
# `_ff_state="$workspace/.fast-fail-counter"` declaration, and the unused
# state.json:fastFailCounters surface is annotated (schema dead-weight
# called out so future contributors don't assume cross-cycle persistence).
#
# Builder freedom: exact wording is not constrained. We require the
# following observable facts to appear in the documentation surface
# (comment block within 10 lines above or 5 lines below the `_ff_state`
# declaration; OR a referenced ADR in docs/architecture/ linked from the
# comment):
#
#   1. Mention that the counter is scoped per-workspace (resets each cycle).
#   2. Mention that cross-cycle persistence via state.json:fastFailCounters
#      is intentionally NOT used (reserved / dead-weight / future).
#   3. Mention the rationale: structural dispatch failures within a single
#      cycle invocation, not cross-cycle aggregation.
set -uo pipefail

REPO_ROOT="${WORKTREE_PATH:-${EVOLVE_PROJECT_ROOT:-$(git rev-parse --show-toplevel 2>/dev/null || pwd)}}"
SUBAGENT_RUN="$REPO_ROOT/scripts/dispatch/subagent-run.sh"

if [ ! -f "$SUBAGENT_RUN" ]; then
    echo "RED cycle-95-fastfail-counter-docs: subagent-run.sh not found at $SUBAGENT_RUN" >&2
    exit 1
fi

# Locate the `_ff_state="$workspace/.fast-fail-counter"` declaration line.
ff_line=$(grep -n '_ff_state="\$workspace/.fast-fail-counter"' "$SUBAGENT_RUN" | head -1 | cut -d: -f1)
if [ -z "$ff_line" ]; then
    echo "RED cycle-95-fastfail-counter-docs: cannot locate _ff_state declaration in $SUBAGENT_RUN" >&2
    exit 1
fi

# Extract a window: 20 lines above and 5 lines below the declaration.
# This is the "documentation surface" Builder is expected to populate.
win_start=$((ff_line - 20))
[ "$win_start" -lt 1 ] && win_start=1
win_end=$((ff_line + 5))
window=$(sed -n "${win_start},${win_end}p" "$SUBAGENT_RUN")

fail=0
errors=""

# (1) per-workspace scoping is acknowledged.
if ! printf '%s' "$window" | grep -qiE 'per[-[:space:]]workspace|workspace[-[:space:]]scoped|per[-[:space:]]cycle|resets[[:space:]]each[[:space:]]cycle|reset[[:space:]]per[[:space:]]cycle|fresh[[:space:]]each[[:space:]]cycle'; then
    errors="${errors}\n  missing per-workspace / per-cycle scoping note near line $ff_line"
    fail=$((fail + 1))
fi

# (2) state.json:fastFailCounters is called out as unused/reserved.
# Accept the literal field name OR a clear "cross-cycle" rejection phrase that
# names the alternative (state.json) explicitly.
if ! printf '%s' "$window" | grep -qiE 'fastFailCounters|state\.json.*(unused|reserved|not[[:space:]]used|dead[-[:space:]]weight)|cross[-[:space:]]cycle.*(rejected|not[[:space:]]used|intentionally[[:space:]]omitted)'; then
    errors="${errors}\n  missing state.json:fastFailCounters / cross-cycle-rejected annotation near line $ff_line"
    fail=$((fail + 1))
fi

# (3) Rationale: structural dispatch failure within one cycle.
if ! printf '%s' "$window" | grep -qiE 'structural[[:space:]]dispatch|structural[[:space:]]failure|within[[:space:]]a[[:space:]]single[[:space:]]cycle|single[-[:space:]]cycle[[:space:]]invocation|cycle[[:space:]]invocation'; then
    errors="${errors}\n  missing structural-dispatch / single-cycle rationale near line $ff_line"
    fail=$((fail + 1))
fi

# (4) Sanity: at least one of the existing structural-failure comments (which
# DO mention "structural dispatch failure") is in the window. This guards
# against the predicate accidentally matching on noise far from the target.
if ! printf '%s' "$window" | grep -qE 'fast-fail|FAST-FAIL|fast_fail|fastfail'; then
    errors="${errors}\n  documentation surface is detached from the fast-fail block (no fast-fail keyword found)"
    fail=$((fail + 1))
fi

if [ "$fail" -gt 0 ]; then
    echo "RED cycle-95-fastfail-counter-docs: $fail issue(s) in documentation surface around line $ff_line"
    printf "%b\n" "$errors" >&2
    exit 1
fi

echo "GREEN cycle-95-fastfail-counter-docs: per-workspace scoping, state.json reserved-status, and single-cycle rationale all documented near line $ff_line"
exit 0
