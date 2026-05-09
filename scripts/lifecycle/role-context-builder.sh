#!/usr/bin/env bash
#
# role-context-builder.sh — v8.56.0 Layer B per-role context filter.
#
# The orchestrator persona dispatches phase agents via subagent-run.sh.
# Pre-v8.56 every agent received the same kitchen-sink context (full
# state.json + scout + build + audit + retrospective). This wasted tokens
# AND diluted attention — Builder doesn't need retrospective theory;
# Auditor doesn't need Scout's raw research notes.
#
# This helper assembles a role-appropriate context block. Orchestrators
# call it instead of dumping every artifact in every prompt:
#
#   bash scripts/lifecycle/role-context-builder.sh <role> <cycle> <workspace_path>
#
# Output: markdown context block on stdout. Orchestrator pipes/concatenates
# this with its task-specific instructions before invoking subagent-run.sh.
#
# Roles (must match agent profile basenames):
#   scout         — carryoverTodos + instinctSummary + intent.md (NO build/audit)
#   triage        — scout-report + carryoverTodos + intent.md (NO build/audit/retro)
#   plan_review   — scout-report + intent.md + carryoverTodos (NO build/retro)
#   tdd           — scout-report + intent.md (NO retro, top-3 instincts only)
#   builder       — scout-report + intent.md + RED test (NO retro, NO full instincts)
#   auditor       — build-report + intent.md + scout-report (NO retro, NO raw research)
#   retrospective — ALL artifacts (it's the synthesizer)
#
# Token cap: EVOLVE_PROMPT_MAX_TOKENS (default 30000). Over-cap emits a
# stderr WARN; the orchestrator should consider summarizing artifacts
# before re-dispatching. Hard cap is informational — does NOT truncate.
#
# Exit codes:
#   0  — context emitted successfully (may include WARN on stderr)
#   2  — unknown role; nothing emitted

set -uo pipefail

__rr_self="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
. "$__rr_self/resolve-roots.sh" 2>/dev/null || {
    # Fallback for test isolation: derive project root from arg 3 if given.
    EVOLVE_PROJECT_ROOT="${EVOLVE_PROJECT_ROOT:-$(pwd)}"
}
unset __rr_self

ROLE="${1:-}"
CYCLE="${2:-}"
WORKSPACE="${3:-}"

usage() {
    cat >&2 <<USG
Usage: role-context-builder.sh <role> <cycle> <workspace_path>

Roles: scout | triage | plan_review | tdd | builder | auditor | retrospective

Env:
  EVOLVE_PROMPT_MAX_TOKENS  — soft cap for context block (default 30000)
  EVOLVE_PROJECT_ROOT       — project root (auto-detected from resolve-roots.sh)
USG
}

[ -n "$ROLE" ] && [ -n "$CYCLE" ] && [ -n "$WORKSPACE" ] || { usage; exit 1; }

STATE="$EVOLVE_PROJECT_ROOT/.evolve/state.json"
MAX_TOKENS="${EVOLVE_PROMPT_MAX_TOKENS:-30000}"

emit_artifact() {
    local label="$1" path="$2"
    if [ -f "$path" ]; then
        echo "## $label"
        echo "(source: $path)"
        echo
        cat "$path"
        echo
    fi
}

emit_state_field() {
    # $1 = jq path expression, $2 = label, $3 = optional limit (head -N)
    local expr="$1" label="$2" limit="${3:-}"
    [ -f "$STATE" ] || return 0
    local content
    content=$(jq -r "$expr" "$STATE" 2>/dev/null || echo "")
    [ -z "$content" ] && return 0
    [ "$content" = "null" ] && return 0
    [ "$content" = "[]" ] && return 0
    echo "## $label"
    if [ -n "$limit" ]; then
        echo "$content" | head -n "$limit"
    else
        echo "$content"
    fi
    echo
}

emit_carryover_todos() {
    [ -f "$STATE" ] || return 0
    local todos
    todos=$(jq -r '.carryoverTodos // [] | length' "$STATE" 2>/dev/null || echo 0)
    [ "$todos" = "0" ] && return 0
    echo "## carryoverTodos"
    # v8.57.0 Layer D: surface cycles_unpicked so consumers (Scout, Triage)
    # see the freshness signal — items closer to threshold lose priority.
    jq -r '.carryoverTodos[] | "- [" + (.priority // "?") + "] " + .id + ": " + .action + " (defer=" + ((.defer_count // 0) | tostring) + ", unpicked=" + ((.cycles_unpicked // 0) | tostring) + ", evidence=" + (.evidence_pointer // "none") + ")"' "$STATE" 2>/dev/null
    echo
}

emit_instincts() {
    # $1 = optional limit (default: all up to N=5 cap)
    local limit="${1:-5}"
    [ -f "$STATE" ] || return 0
    local count
    count=$(jq -r '.instinctSummary // [] | length' "$STATE" 2>/dev/null || echo 0)
    [ "$count" = "0" ] && return 0
    echo "## instinctSummary (top $limit, most-recent)"
    jq -r --argjson n "$limit" '.instinctSummary[-$n:][] | "- " + .id + " [" + (.errorCategory // "?") + "]: " + (.pattern // "?") + " (confidence=" + ((.confidence // 0) | tostring) + ")"' "$STATE" 2>/dev/null
    echo
}

# Header common to all roles.
header_block() {
    cat <<EOF
---
ROLE CONTEXT BLOCK (assembled by role-context-builder.sh v8.56.0)
role: $ROLE
cycle: $CYCLE
workspace: $WORKSPACE
---

EOF
}

# ---- Per-role assembly -----------------------------------------------------
emit_for_role() {
    case "$ROLE" in
        scout)
            header_block
            emit_artifact "Intent" "$WORKSPACE/intent.md"
            emit_carryover_todos
            emit_instincts 5
            ;;
        triage)
            # Layer C: Triage gets the backlog plus carryover context to choose top-N.
            header_block
            emit_artifact "Intent" "$WORKSPACE/intent.md"
            emit_artifact "Scout backlog" "$WORKSPACE/scout-report.md"
            emit_carryover_todos
            ;;
        plan_review)
            header_block
            emit_artifact "Intent" "$WORKSPACE/intent.md"
            emit_artifact "Scout report" "$WORKSPACE/scout-report.md"
            emit_carryover_todos
            ;;
        tdd)
            header_block
            emit_artifact "Intent" "$WORKSPACE/intent.md"
            emit_artifact "Scout selected backlog" "$WORKSPACE/scout-report.md"
            # tdd gets a *small* slice of instincts — top-3 only
            emit_instincts 3
            ;;
        builder)
            header_block
            emit_artifact "Intent" "$WORKSPACE/intent.md"
            emit_artifact "Scout selected backlog" "$WORKSPACE/scout-report.md"
            # RED test file (TDD output) if present
            emit_artifact "RED test (must turn GREEN)" "$WORKSPACE/red-test.md"
            # Builder gets NO retrospective theory and NO whole instinctSummary.
            ;;
        auditor)
            header_block
            emit_artifact "Intent (acceptance criteria)" "$WORKSPACE/intent.md"
            emit_artifact "Build report" "$WORKSPACE/build-report.md"
            # Scout-report is included as a *relevant subset* — the orchestrator
            # would normally trim to acceptance-relevant sections; here we ship
            # the whole file because trimming requires LLM judgement. Token cap
            # warns operator if this overflows.
            emit_artifact "Scout report (acceptance scope)" "$WORKSPACE/scout-report.md"
            ;;
        retrospective)
            # Synthesizer: gets every phase artifact.
            header_block
            emit_artifact "Intent" "$WORKSPACE/intent.md"
            emit_artifact "Scout report" "$WORKSPACE/scout-report.md"
            emit_artifact "Build report" "$WORKSPACE/build-report.md"
            emit_artifact "Audit report" "$WORKSPACE/audit-report.md"
            emit_artifact "Triage decision (if Layer C ran)" "$WORKSPACE/triage-decision.md"
            emit_carryover_todos
            emit_instincts 5
            ;;
        *)
            echo "ERROR: unknown role: $ROLE" >&2
            echo "Valid roles: scout|triage|plan_review|tdd|builder|auditor|retrospective" >&2
            exit 2
            ;;
    esac
}

# Capture output to a tmp file so we can measure size for the cap guard,
# THEN emit. (Streaming would bypass the guard.)
TMP_OUT=$(mktemp)
trap 'rm -f "$TMP_OUT"' EXIT

emit_for_role > "$TMP_OUT"

# Token estimation: 1 token ≈ 4 bytes (English-text upper bound).
BYTES=$(wc -c < "$TMP_OUT" | tr -d ' ')
EST_TOKENS=$((BYTES / 4))
if [ "$EST_TOKENS" -gt "$MAX_TOKENS" ]; then
    echo "[role-context-builder] WARN: context for role '$ROLE' exceeds max tokens ($EST_TOKENS > $MAX_TOKENS). Consider summarizing artifacts. Context emitted as-is; orchestrator should trim before invoking subagent." >&2
fi

cat "$TMP_OUT"
